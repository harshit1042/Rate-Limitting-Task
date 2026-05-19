#!/usr/bin/env bash
# End-to-end smoke test against a running server on BASE_URL (default http://localhost:8080).
set -euo pipefail

BASE_URL="${BASE_URL:-http://localhost:8080}"
PASS=0
FAIL=0

assert_status() {
  local name="$1" expected="$2" actual="$3"
  if [[ "$actual" == "$expected" ]]; then
    echo "  OK   $name (HTTP $actual)"
    PASS=$((PASS + 1))
  else
    echo "  FAIL $name (expected HTTP $expected, got $actual)"
    FAIL=$((FAIL + 1))
  fi
}

assert_body_contains() {
  local name="$1" needle="$2" body="$3"
  if echo "$body" | grep -q "$needle"; then
    echo "  OK   $name (body contains '$needle')"
    PASS=$((PASS + 1))
  else
    echo "  FAIL $name (body missing '$needle')"
    echo "       body: ${body:0:200}"
    FAIL=$((FAIL + 1))
  fi
}

assert_body_not_contains() {
  local name="$1" needle="$2" body="$3"
  if echo "$body" | grep -q "$needle"; then
    echo "  FAIL $name (body should not contain '$needle')"
    FAIL=$((FAIL + 1))
  else
    echo "  OK   $name (no '$needle' in list response)"
    PASS=$((PASS + 1))
  fi
}

echo "=== E2E smoke: $BASE_URL ==="

# --- Part 1: rate limit ---
echo ""
echo "--- Part 1: POST /request & GET /stats ---"
USER="e2e-user-$$"
for i in 1 2 3 4 5; do
  code=$(curl -s -o /dev/null -w "%{http_code}" -X POST "$BASE_URL/request" \
    -H 'Content-Type: application/json' \
    -d "{\"user_id\":\"$USER\",\"payload\":{\"n\":$i}}")
  assert_status "accept request $i/5" "200" "$code"
done
code=$(curl -s -o /dev/null -w "%{http_code}" -X POST "$BASE_URL/request" \
  -H 'Content-Type: application/json' \
  -d "{\"user_id\":\"$USER\",\"payload\":{\"n\":6}}")
assert_status "6th request rate limited" "429" "$code"

body=$(curl -s -X POST "$BASE_URL/request" \
  -H 'Content-Type: application/json' \
  -d "{\"user_id\":\"$USER\",\"payload\":{\"n\":6}}")
assert_body_contains "429 JSON error message" "Rate limit exceeded" "$body"

code=$(curl -s -o /dev/null -w "%{http_code}" -X POST "$BASE_URL/request" \
  -H 'Content-Type: application/json' \
  -d '{"user_id":"","payload":{}}')
assert_status "empty user_id" "400" "$code"

code=$(curl -s -o /dev/null -w "%{http_code}" -X POST "$BASE_URL/request" \
  -H 'Content-Type: application/json' \
  -d 'not-json')
assert_status "invalid JSON" "400" "$code"

stats=$(curl -s "$BASE_URL/stats")
assert_body_contains "stats has user" "\"user_id\":\"$USER\"" "$stats"
assert_body_contains "stats requests_in_current_window" "requests_in_current_window" "$stats"

# --- Part 2: products ---
echo ""
echo "--- Part 2: Product catalog ---"
SKU="SKU-E2E-$$"
create=$(curl -s -w "\n%{http_code}" -X POST "$BASE_URL/products" \
  -H 'Content-Type: application/json' \
  -d "{\"name\":\"Widget\",\"sku\":\"$SKU\",\"image_urls\":[\"https://cdn.example.com/$SKU/1.jpg\"],\"video_urls\":[]}")
create_body=$(echo "$create" | sed '$d')
create_code=$(echo "$create" | tail -1)
assert_status "POST /products" "201" "$create_code"
assert_body_contains "created product has id" "\"id\"" "$create_body"

PRODUCT_ID=$(echo "$create_body" | sed -n 's/.*"id"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p' | head -1)
if [[ -z "$PRODUCT_ID" ]]; then
  echo "  FAIL could not parse product id"
  FAIL=$((FAIL + 1))
else
  echo "  OK   parsed product id $PRODUCT_ID"
  PASS=$((PASS + 1))
fi

code=$(curl -s -o /dev/null -w "%{http_code}" -X POST "$BASE_URL/products" \
  -H 'Content-Type: application/json' \
  -d "{\"name\":\"Dup\",\"sku\":\"$SKU\"}")
assert_status "duplicate sku" "409" "$code"

detail=$(curl -s -w "\n%{http_code}" "$BASE_URL/products/$PRODUCT_ID")
detail_body=$(echo "$detail" | sed '$d')
detail_code=$(echo "$detail" | tail -1)
assert_status "GET /products/{id}" "200" "$detail_code"
assert_body_contains "detail has image_urls" "image_urls" "$detail_body"

list=$(curl -s -w "\n%{http_code}" "$BASE_URL/products?limit=5&offset=0")
list_body=$(echo "$list" | sed '$d')
list_code=$(echo "$list" | tail -1)
assert_status "GET /products list" "200" "$list_code"
assert_body_contains "list has image_count" "image_count" "$list_body"
assert_body_not_contains "list omits image_urls" "image_urls" "$list_body"

append=$(curl -s -w "\n%{http_code}" -X POST "$BASE_URL/products/$PRODUCT_ID/media" \
  -H 'Content-Type: application/json' \
  -d '{"image_urls":["https://cdn.example.com/'"$SKU"'/2.jpg"]}')
append_code=$(echo "$append" | tail -1)
assert_status "POST media append" "200" "$append_code"

code=$(curl -s -o /dev/null -w "%{http_code}" "$BASE_URL/products/00000000-0000-0000-0000-000000000099")
assert_status "unknown product" "404" "$code"

code=$(curl -s -o /dev/null -w "%{http_code}" -X POST "$BASE_URL/products/$PRODUCT_ID/media" \
  -H 'Content-Type: application/json' \
  -d '{}')
assert_status "empty media body" "400" "$code"

code=$(curl -s -o /dev/null -w "%{http_code}" -X POST "$BASE_URL/products" \
  -H 'Content-Type: application/json' \
  -d '{"name":"Bad","sku":"SKU-BAD","image_urls":["ftp://bad.com/x.jpg"]}')
assert_status "invalid url scheme" "400" "$code"

echo ""
echo "=== Results: $PASS passed, $FAIL failed ==="
[[ "$FAIL" -eq 0 ]]
