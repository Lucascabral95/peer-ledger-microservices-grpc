# Loki Queries (LogQL)

Este archivo define consultas operativas recomendadas para `Peer Ledger`.
Todas las queries asumen labels `service`, `container`, `stream` y logs de Docker.

## 1) Estado general y volumen

### 1.1 Stream global en vivo
```logql
{service=~".+"}
```
Uso: inspeccion en tiempo real de todo el stack.

### 1.2 Top servicios por volumen (5m)
```logql
topk(5, sum by (service) (count_over_time({service=~".+"}[5m])))
```
Uso: detectar servicios ruidosos o picos anormales de logs.

### 1.3 Volumen por servicio y stream (stdout/stderr)
```logql
sum by (service, stream) (count_over_time({service=~".+"}[10m]))
```
Uso: separar tráfico normal (`stdout`) de errores (`stderr`).

## 2) Errores críticos y estabilidad

### 2.1 Errores por servicio (5m)
```logql
sum by (service) (count_over_time({service=~".+"} |~ "(?i)\\berror\\b"[5m]))
```
Uso: heatmap de fallos por servicio.

### 2.2 Panics / fatals
```logql
{service=~".+"} |~ "(?i)panic|fatal"
```
Uso: incidentes críticos de ejecución.

### 2.3 Timeouts y conectividad
```logql
{service=~".+"} |~ "(?i)deadline exceeded|timeout|connection refused|context deadline exceeded"
```
Uso: detectar degradación de red o servicios dependientes caídos.

### 2.4 Errores gRPC
```logql
{service=~".+"} |= "rpc error"
```
Uso: rastrear fallos de contratos internos entre microservicios.

## 3) Gateway y tráfico HTTP

### 3.1 Requests de transferencias
```logql
{service=~"project-gateway-.*"} |= "POST /transfers"
```
Uso: seguimiento operativo del endpoint crítico.

### 3.2 Bloqueos por rate limit
```logql
{service=~"project-gateway-.*"} |= "rate limit exceeded"
```
Uso: validar si límites son demasiado agresivos.

### 3.3 Respuestas 5xx registradas en gateway
```logql
{service=~"project-gateway-.*"} |~ "\\s5[0-9][0-9]\\s"
```
Uso: investigación de errores HTTP server-side.

## 4) Flujo de negocio (wallet, fraude, auditoría)

### 4.1 Fallos de provisión de wallet al registrarse
```logql
{service=~"project-gateway-.*"} |= "wallet provisioning failed"
```
Uso: detectar inconsistencias `register -> create wallet`.

### 4.2 Transferencias bloqueadas por fraude
```logql
{service=~"project-gateway-.*"} |= "transfer blocked by fraud service"
```
Uso: ver impacto real de reglas antifraude.

### 4.3 Errores al registrar auditoría
```logql
{service=~"project-gateway-.*"} |= "failed to record audit transaction"
```
Uso: identificar escenarios con dinero movido pero auditoría fallida.

### 4.4 Fondos insuficientes (wallet)
```logql
{service=~".+"} |= "insufficient funds"
```
Uso: medir fricción de producto y rechazos de negocio.

## 5) Seguridad y autenticación

### 5.1 Errores de token/authorization
```logql
{service=~"project-gateway-.*"} |~ "(?i)authorization|unauthorized|invalid or expired token"
```
Uso: detectar abuso o problemas de expiración/JWT.

### 5.2 Reintentos/ataques de login
```logql
sum by (container) (count_over_time({service=~"project-gateway-.*"} |= "POST /auth/login"[5m]))
```
Uso: identificar patrones anómalos en autenticación.

## 6) Observabilidad del stack de observabilidad

### 6.1 Salud de Promtail
```logql
{service=~"project-promtail-.*"} |~ "(?i)error|warn"
```
Uso: verificar problemas de scraping/envío hacia Loki.

### 6.2 Salud de Alertmanager
```logql
{service=~"project-alertmanager-.*"} |~ "(?i)error|smtp|notify|failed"
```
Uso: depurar fallos de notificación de alertas.

### 6.3 Salud de Loki
```logql
{service=~"project-loki-.*"} |~ "(?i)error|ingester|compactor"
```
Uso: detectar problemas internos del backend de logs.

## 7) Consultas de corte (incident response)

### 7.1 Últimos 15 minutos, solo errores
```logql
{service=~".+"} |~ "(?i)\\berror\\b|panic|fatal"
```
Uso: triage inicial de incidente.

### 7.2 Correlación rápida por servicio crítico
```logql
{service=~"project-(gateway|wallet-service|transaction-service)-.*"} |~ "(?i)error|timeout|rpc error|insufficient funds"
```
Uso: seguimiento rápido del flujo `/transfers`.

## 8) Notas de operación

- Si cambia el nombre del proyecto/compose, ajustá regex `project-...`.
- Para dashboards, usar `count_over_time(...)` en paneles tipo time series o stat.
- Para explorar logs crudos, usar query de streams (`{...}` + filtros `|=`, `|~`).
