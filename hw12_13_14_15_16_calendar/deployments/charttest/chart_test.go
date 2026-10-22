// Package charttest checks rendered Kubernetes contracts without a running cluster.
package charttest

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"gopkg.in/yaml.v3"
)

const setFlag = "--set"

type document struct {
	Kind     string `yaml:"kind"`
	Metadata struct {
		Name      string `yaml:"name"`
		Namespace string `yaml:"namespace"`
	} `yaml:"metadata"`
	Spec map[string]any `yaml:"spec"`
}

func render(t *testing.T, args ...string) ([]document, error) {
	t.Helper()
	helm := os.Getenv("CALENDAR_HELM")
	if helm == "" {
		t.Skip("set CALENDAR_HELM to run Helm contract checks")
	}
	command := make([]string, 0, 5+len(args))
	command = append(command, "template", "alternate", "../calendar-chart", "--namespace", "other-namespace")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, helm, append(command, args...)...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return nil, errors.New(string(out))
	}
	decoder := yaml.NewDecoder(bytes.NewReader(out))
	var documents []document
	for {
		var d document
		err := decoder.Decode(&d)
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		if d.Kind != "" {
			documents = append(documents, d)
		}
	}
	return documents, nil
}

func nested(t *testing.T, object map[string]any, keys ...string) any {
	t.Helper()
	var value any = object
	for _, key := range keys {
		mapping, ok := value.(map[string]any)
		if !ok {
			t.Fatalf("expected mapping at %s: %v", key, value)
		}
		value = mapping[key]
	}
	return value
}

func TestDefaultChartContracts(t *testing.T) {
	documents, err := render(t, setFlag, "images.api.tag=custom")
	if err != nil {
		t.Fatal(err)
	}
	counts := map[string]int{}
	for _, d := range documents {
		counts[d.Kind]++
		if d.Metadata.Namespace != "other-namespace" || !strings.HasPrefix(d.Metadata.Name, "alternate-") {
			t.Fatalf("release/namespace escaped: %+v", d.Metadata)
		}
		checkResourceContract(t, d)
	}
	for kind, want := range map[string]int{"Deployment": 3, "StatefulSet": 2, "Service": 5, "Ingress": 1, "Job": 0} {
		if counts[kind] != want {
			t.Fatalf("%s: got %d want %d", kind, counts[kind], want)
		}
	}
}

func checkResourceContract(t *testing.T, d document) {
	t.Helper()
	if d.Kind == "Service" && d.Metadata.Name != "alternate-api" {
		if d.Spec["clusterIP"] != "None" {
			t.Fatalf("not headless: %s", d.Metadata.Name)
		}
		if strings.Contains(d.Metadata.Name, "sender") || strings.Contains(d.Metadata.Name, "scheduler") {
			if _, ok := d.Spec["ports"]; ok {
				t.Fatal("worker has invented inbound port")
			}
		}
	}
	if d.Kind == "Deployment" {
		selector := nested(t, d.Spec, "selector", "matchLabels").(map[string]any)
		labels := nested(t, d.Spec, "template", "metadata", "labels").(map[string]any)
		for key, value := range selector {
			if labels[key] != value {
				t.Fatalf("selector mismatch: %s", key)
			}
		}
		if d.Metadata.Name == "alternate-api" {
			containers := nested(t, d.Spec, "template", "spec", "containers").([]any)
			if containers[0].(map[string]any)["image"] != "calendar-api:custom" {
				t.Fatal("image override ignored")
			}
		}
	}
}

func TestExternalServicesAndDisabledIngress(t *testing.T) {
	documents, err := render(t, setFlag, "postgres.enabled=false,rabbitmq.enabled=false,ingress.enabled=false",
		setFlag, "connections.existingSecret=my-connections")
	if err != nil {
		t.Fatal(err)
	}
	for _, d := range documents {
		if d.Kind == "Ingress" || d.Kind == "StatefulSet" || d.Kind == "Secret" {
			t.Fatalf("unexpected %s", d.Kind)
		}
	}
}

func TestInvalidChartValues(t *testing.T) {
	for _, args := range [][]string{
		{setFlag, "service.httpPort=0"},
		{setFlag, "replicas.api=0"},
		{setFlag, "config.schedulerInterval=0s"},
		{setFlag, "postgres.password=unsafe@password"},
		{setFlag, "postgres.enabled=false"},
		{setFlag, "rabbitmq.enabled=false"},
		{setFlag, "service.httpPort=50051,service.grpcPort=50051"},
	} {
		if _, err := render(t, args...); err == nil {
			t.Fatalf("accepted invalid values: %v", args)
		}
	}
}

func TestGeneratedManifest(t *testing.T) {
	helm := os.Getenv("CALENDAR_HELM")
	if helm == "" {
		t.Skip("set CALENDAR_HELM to check generated manifest")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, helm, "template", "calendar", "../calendar-chart", "--namespace", "calendar-hw16")
	out, err := cmd.Output()
	if err != nil {
		t.Fatal(err)
	}
	saved, err := os.ReadFile(filepath.Join("..", "kubernetes", "calendar.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(out, saved) {
		t.Fatal("generated manifests drifted; run make k8s-render")
	}
}
