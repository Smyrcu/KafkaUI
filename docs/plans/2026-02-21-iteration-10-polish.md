# Iteration 10: Polish — Dashboard & Overview

## Scope
Multi-cluster dashboard with health status, aggregate statistics, and cluster overview cards. Replaces the clusters list as the landing page.

## Architecture

### Backend
- `internal/api/handlers/dashboard.go` — Dashboard handler with parallel cluster health checks
- `ClusterOverview` type: name, bootstrapServers, brokerCount, topicCount, consumerGroupCount, status

### API Endpoints
| Method | Path | Description |
|--------|------|-------------|
| GET | `/dashboard` | All clusters overview |
| GET | `/clusters/{clusterName}/overview` | Single cluster overview |

### Frontend
- `DashboardPage` — Summary bar (totals) + cluster cards grid with status badges
- Auto-refresh every 30 seconds
- Links to cluster sub-pages from dashboard cards

### Health Status
- healthy: all APIs responding
- degraded: some APIs failing
- unreachable: cannot connect to Kafka

## Files
- `internal/api/handlers/dashboard.go` — new
- `frontend/src/pages/DashboardPage.tsx` — new
- Plus shared file updates (api.ts, router.go, App.tsx)
