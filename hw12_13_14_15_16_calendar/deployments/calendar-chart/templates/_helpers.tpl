{{- define "calendar.labels" -}}
app.kubernetes.io/name: calendar
app.kubernetes.io/instance: {{ .Release.Name }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}
{{- define "calendar.secret" -}}
{{- default (printf "%s-connections" .Release.Name) .Values.connections.existingSecret -}}
{{- end }}
{{- define "calendar.databaseDSN" -}}
{{- if .Values.connections.databaseDSN -}}
{{- .Values.connections.databaseDSN -}}
{{- else if .Values.postgres.enabled -}}
{{- printf "postgres://%s:%s@%s-postgres:5432/%s?sslmode=disable" .Values.postgres.user .Values.postgres.password .Release.Name .Values.postgres.database -}}
{{- else -}}
{{- fail "External PostgreSQL requires connections.databaseDSN or connections.existingSecret" -}}
{{- end -}}
{{- end }}
{{- define "calendar.rabbitmqURL" -}}
{{- if .Values.connections.rabbitmqURL -}}
{{- .Values.connections.rabbitmqURL -}}
{{- else if .Values.rabbitmq.enabled -}}
{{- printf "amqp://%s:%s@%s-rabbitmq:5672/" .Values.rabbitmq.user .Values.rabbitmq.password .Release.Name -}}
{{- else -}}
{{- fail "External RabbitMQ requires connections.rabbitmqURL or connections.existingSecret" -}}
{{- end -}}
{{- end }}
