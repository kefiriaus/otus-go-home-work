package hw09structvalidator

import (
	"encoding/json"
	"errors"
	"fmt"
	"testing"
)

type UserRole string

type (
	User struct {
		ID     string `json:"id" validate:"len:36"`
		Name   string
		Age    int             `validate:"min:18|max:50"`
		Email  string          `validate:"regexp:^\\w+@\\w+\\.\\w+$"`
		Role   UserRole        `validate:"in:admin,stuff"`
		Phones []string        `validate:"len:11"`
		meta   json.RawMessage //nolint:unused
	}

	App struct {
		Version string `validate:"len:5"`
	}

	Token struct {
		Header    []byte
		Payload   []byte
		Signature []byte
	}

	Response struct {
		Code int    `validate:"in:200,404,500"`
		Body string `json:"omitempty"`
	}

	BadRule struct {
		Value int `validate:"min:abc"`
	}

	UnknownRuleStruct struct {
		Value string `validate:"length:5"`
	}

	MalformedTag struct {
		Value string `validate:"len"`
	}

	UnsupportedTypeStruct struct {
		Value float64 `validate:"min:1"`
	}

	Profile struct {
		App  App      `validate:"nested"`
		Resp Response `validate:"nested"`
	}
)

func assertValidationErrors(t *testing.T, err error, expected ValidationErrors) {
	t.Helper()

	var actual ValidationErrors
	if !errors.As(err, &actual) {
		t.Fatalf("expected ValidationErrors, got %T: %v", err, err)
	}
	if len(actual) != len(expected) {
		t.Fatalf("expected %d validation errors, got %d: %v", len(expected), len(actual), actual)
	}
	for i := range expected {
		if actual[i].Field != expected[i].Field {
			t.Errorf("error %d: expected field %q, got %q", i, expected[i].Field, actual[i].Field)
		}
		if !errors.Is(actual[i].Err, expected[i].Err) {
			t.Errorf("error %d: expected %v, got %v", i, expected[i].Err, actual[i].Err)
		}
	}
}

func assertError(t *testing.T, err, expected error) {
	t.Helper()

	if expected == nil {
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		return
	}

	var ve ValidationErrors
	if errors.As(expected, &ve) {
		assertValidationErrors(t, err, ve)
		return
	}

	if !errors.Is(err, expected) {
		t.Fatalf("expected %v, got %v", expected, err)
	}
}

func TestValidate(t *testing.T) { //nolint:funlen
	tests := []struct {
		name        string
		in          interface{}
		expectedErr error
	}{
		{
			name: "valid user",
			in: User{
				ID:     "123456789012345678901234567890123456",
				Name:   "John",
				Age:    33,
				Email:  "john@example.com",
				Role:   "admin",
				Phones: []string{"12345678901"},
			},
			expectedErr: nil,
		},
		{
			name: "user with all fields invalid",
			in: User{
				ID:     "too-short",
				Age:    17,
				Email:  "not-an-email",
				Role:   "guest",
				Phones: []string{"12345678901", "123"},
			},
			expectedErr: ValidationErrors{
				{Field: "ID", Err: ErrLen},
				{Field: "Age", Err: ErrMin},
				{Field: "Email", Err: ErrRegexp},
				{Field: "Role", Err: ErrIn},
				{Field: "Phones", Err: ErrLen},
			},
		},
		{
			name: "age above max",
			in: User{
				ID:    "123456789012345678901234567890123456",
				Age:   51,
				Email: "john@example.com",
				Role:  "stuff",
			},
			expectedErr: ValidationErrors{
				{Field: "Age", Err: ErrMax},
			},
		},
		{
			name:        "valid app",
			in:          App{Version: "1.0.0"},
			expectedErr: nil,
		},
		{
			name:        "invalid app version",
			in:          App{Version: "1.0"},
			expectedErr: ValidationErrors{{Field: "Version", Err: ErrLen}},
		},
		{
			name:        "struct without validate tags",
			in:          Token{Header: []byte("h"), Payload: []byte("p"), Signature: []byte("s")},
			expectedErr: nil,
		},
		{
			name:        "valid response",
			in:          Response{Code: 404, Body: "not found"},
			expectedErr: nil,
		},
		{
			name:        "invalid response code",
			in:          Response{Code: 503},
			expectedErr: ValidationErrors{{Field: "Code", Err: ErrIn}},
		},
		{
			name:        "pointer to struct is accepted",
			in:          &App{Version: "1.0"},
			expectedErr: ValidationErrors{{Field: "Version", Err: ErrLen}},
		},
		{
			name:        "unicode length is counted in runes",
			in:          App{Version: "привет"},
			expectedErr: ValidationErrors{{Field: "Version", Err: ErrLen}},
		},
		{
			name: "nested structs",
			in:   Profile{App: App{Version: "1.0"}, Resp: Response{Code: 503}},
			expectedErr: ValidationErrors{
				{Field: "App.Version", Err: ErrLen},
				{Field: "Resp.Code", Err: ErrIn},
			},
		},
		{
			name:        "not a struct: int",
			in:          42,
			expectedErr: ErrNotStruct,
		},
		{
			name:        "not a struct: string",
			in:          "hello",
			expectedErr: ErrNotStruct,
		},
		{
			name:        "not a struct: nil",
			in:          nil,
			expectedErr: ErrNotStruct,
		},
		{
			name:        "invalid rule parameter",
			in:          BadRule{Value: 1},
			expectedErr: ErrInvalidRuleParam,
		},
		{
			name:        "unknown rule",
			in:          UnknownRuleStruct{Value: "x"},
			expectedErr: ErrUnknownRule,
		},
		{
			name:        "malformed tag",
			in:          MalformedTag{Value: "x"},
			expectedErr: ErrInvalidRule,
		},
		{
			name:        "unsupported field type",
			in:          UnsupportedTypeStruct{Value: 1.5},
			expectedErr: ErrUnsupportedType,
		},
	}

	for i, tt := range tests {
		t.Run(fmt.Sprintf("case %d: %s", i, tt.name), func(t *testing.T) {
			t.Parallel()
			assertError(t, Validate(tt.in), tt.expectedErr)
		})
	}
}
