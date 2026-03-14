#!/bin/bash
set -euo pipefail

# ============================================================================
# 冒烟测试：验证 Go 服务 + 全链路 API
# 用法: ./deploy/scripts/smoke-test.sh [SERVICE_URL]
# ============================================================================

SERVICE_URL="${1:-http://localhost:8090}"
PASS=0
FAIL=0
TOTAL=0

ok() { PASS=$((PASS+1)); TOTAL=$((TOTAL+1)); echo "  ✅ $1"; }
fail() { FAIL=$((FAIL+1)); TOTAL=$((TOTAL+1)); echo "  ❌ $1: $2"; }

echo "============================================"
echo "  冒烟测试"
echo "  URL: $SERVICE_URL"
echo "============================================"
echo ""

# ---- 1. Health ----
echo "--- [1/10] Health Check ---"
RES=$(curl -sf "$SERVICE_URL/health" 2>/dev/null || echo "FAIL")
if echo "$RES" | grep -q '"ok"'; then ok "health"; else fail "health" "$RES"; fi

# ---- 2. Register Agent ----
echo "--- [2/10] Register Agent ---"
RES=$(curl -sf -X POST "$SERVICE_URL/api/v1/agents/register" \
  -H 'Content-Type: application/json' \
  -d '{"id":"smoke-arch","display_name":"烟测架构师","emoji":"🏗️","role":"architect","skills":["design"]}' 2>/dev/null || echo "FAIL")
if echo "$RES" | grep -q 'smoke-arch'; then ok "register agent"; else fail "register agent" "$RES"; fi

# ---- 3. List Agents ----
echo "--- [3/10] List Agents ---"
RES=$(curl -sf "$SERVICE_URL/api/v1/agents" 2>/dev/null || echo "FAIL")
if echo "$RES" | grep -q 'smoke-arch'; then ok "list agents"; else fail "list agents" "$RES"; fi

# ---- 4. Create Project ----
echo "--- [4/10] Create Project ---"
GRP="oc_smoke_$(date +%s)"
RES=$(curl -sf -X POST "$SERVICE_URL/api/v1/projects" \
  -H 'Content-Type: application/json' \
  -d "{\"name\":\"烟测项目\",\"group_id\":\"$GRP\",\"team_agents\":[\"smoke-arch\"],\"tech_stack\":[\"Go\"]}" 2>/dev/null || echo "FAIL")
PROJ_ID=$(echo "$RES" | grep -o '"id":"[^"]*"' | head -1 | cut -d'"' -f4)
if [ -n "$PROJ_ID" ]; then ok "create project ($PROJ_ID)"; else fail "create project" "$RES"; fi

# ---- 5. Create Task ----
echo "--- [5/10] Create Task ---"
RES=$(curl -sf -X POST "$SERVICE_URL/api/v1/tasks" \
  -H 'Content-Type: application/json' \
  -d "{\"project_id\":\"$PROJ_ID\",\"title\":\"烟测任务\",\"assignee\":\"smoke-arch\",\"assigned_by\":\"system\",\"priority\":\"P1\"}" 2>/dev/null || echo "FAIL")
TASK_ID=$(echo "$RES" | grep -o '"id":"[^"]*"' | head -1 | cut -d'"' -f4)
if [ -n "$TASK_ID" ]; then ok "create task ($TASK_ID)"; else fail "create task" "$RES"; fi

# ---- 6. Update Task (assigned) ----
echo "--- [6/10] Update Task → assigned ---"
RES=$(curl -sf -X PATCH "$SERVICE_URL/api/v1/tasks/$TASK_ID" \
  -H 'Content-Type: application/json' \
  -d '{"status":"assigned"}' 2>/dev/null || echo "FAIL")
if echo "$RES" | grep -q 'updated'; then ok "task → assigned"; else fail "task → assigned" "$RES"; fi

# ---- 7. Update Task (in_progress) ----
echo "--- [7/10] Update Task → in_progress ---"
RES=$(curl -sf -X PATCH "$SERVICE_URL/api/v1/tasks/$TASK_ID" \
  -H 'Content-Type: application/json' \
  -d '{"status":"in_progress"}' 2>/dev/null || echo "FAIL")
if echo "$RES" | grep -q 'updated'; then ok "task → in_progress"; else fail "task → in_progress" "$RES"; fi

# ---- 8. Save Decision ----
echo "--- [8/10] Save Decision ---"
RES=$(curl -sf -X POST "$SERVICE_URL/api/v1/projects/$PROJ_ID/memory" \
  -H 'Content-Type: application/json' \
  -d '{"category":"decision","content":"使用 K8s 部署","pinned":true,"created_by":"smoke-arch"}' 2>/dev/null || echo "FAIL")
if echo "$RES" | grep -q '"id"'; then ok "save decision"; else fail "save decision" "$RES"; fi

# ---- 9. Get Context ----
echo "--- [9/10] Get Context ---"
RES=$(curl -sf "$SERVICE_URL/api/v1/projects/$PROJ_ID/context?agent_id=smoke-arch" 2>/dev/null || echo "FAIL")
if echo "$RES" | grep -q '烟测项目'; then ok "get context"; else fail "get context" "$RES"; fi

# ---- 10. Dashboard ----
echo "--- [10/10] Dashboard ---"
RES=$(curl -sf "$SERVICE_URL/api/v1/dashboard/overview" 2>/dev/null || echo "FAIL")
if echo "$RES" | grep -q 'projects'; then ok "dashboard"; else fail "dashboard" "$RES"; fi

# ---- Cleanup ----
curl -sf -X DELETE "$SERVICE_URL/api/v1/agents/smoke-arch" >/dev/null 2>&1 || true

# ---- Summary ----
echo ""
echo "============================================"
echo "  结果: $PASS/$TOTAL 通过"
if [ $FAIL -gt 0 ]; then
  echo "  ❌ $FAIL 个测试失败"
  exit 1
else
  echo "  ✅ 全部通过"
fi
echo "============================================"
