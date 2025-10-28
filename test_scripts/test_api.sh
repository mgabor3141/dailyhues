#!/bin/bash

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

BASE_URL="${BASE_URL:-http://localhost:8080}"

echo -e "${BLUE}🎨 Wallpaper Highlight API Test Suite${NC}\n"

# Function to print test result
print_result() {
    if [ $1 -eq 0 ]; then
        echo -e "${GREEN}✓${NC} $2"
    else
        echo -e "${RED}✗${NC} $2"
        exit 1
    fi
}

# Test 1: Health Check
echo -e "${YELLOW}Test 1: Health Check${NC}"
response=$(curl -s -w "\n%{http_code}" "$BASE_URL/health")
http_code=$(echo "$response" | tail -n1)
body=$(echo "$response" | head -n-1)

if [ "$http_code" = "200" ]; then
    print_result 0 "Health endpoint returned 200"
    echo -e "   Response: $body\n"
else
    print_result 1 "Health endpoint failed (HTTP $http_code)"
fi

# Test 2: Get today's colors (no date parameter)
echo -e "${YELLOW}Test 2: Get Today's Colors (No Date Parameter)${NC}"
response=$(curl -s -w "\n%{http_code}" "$BASE_URL/api/colors")
http_code=$(echo "$response" | tail -n1)
body=$(echo "$response" | head -n-1)

if [ "$http_code" = "200" ]; then
    print_result 0 "API returned 200 for today's date"

    # Check if response contains required fields
    if echo "$body" | jq -e '.date and .images and .colors and .cached_at' > /dev/null 2>&1; then
        print_result 0 "Response contains required fields"
    else
        print_result 1 "Response missing required fields"
    fi

    # Check if images is an object
    if echo "$body" | jq -e '.images | type == "object"' > /dev/null 2>&1; then
        images_count=$(echo "$body" | jq '.images | length')
        print_result 0 "Images object contains $images_count sizes"
    else
        print_result 1 "Images is not an object"
    fi

    # Check if colors is an object
    if echo "$body" | jq -e '.colors | type == "object"' > /dev/null 2>&1; then
        colors_count=$(echo "$body" | jq '.colors | length')
        print_result 0 "Colors object contains $colors_count named colors"
    else
        print_result 1 "Colors is not an object"
    fi

    echo -e "   Response: $body\n"
else
    print_result 1 "API request failed (HTTP $http_code)"
fi

# Test 3: Get specific date
echo -e "${YELLOW}Test 3: Get Colors for Specific Date${NC}"
yesterday=$(date -d "yesterday" +%Y-%m-%d 2>/dev/null || date -v-1d +%Y-%m-%d 2>/dev/null)
response=$(curl -s -w "\n%{http_code}" "$BASE_URL/api/colors?date=$yesterday")
http_code=$(echo "$response" | tail -n1)
body=$(echo "$response" | head -n-1)

if [ "$http_code" = "200" ]; then
    print_result 0 "API returned 200 for date: $yesterday"

    # Verify the date in response matches
    response_date=$(echo "$body" | jq -r '.date')
    if [ "$response_date" = "$yesterday" ]; then
        print_result 0 "Response date matches request ($yesterday)"
    else
        print_result 1 "Response date mismatch (expected: $yesterday, got: $response_date)"
    fi

    echo -e "   Response: $body\n"
else
    echo -e "${YELLOW}⚠${NC}  Note: This might fail if OPENROUTER_API_KEY is not set\n"
    print_result 1 "API request failed (HTTP $http_code)"
fi

# Test 4: Invalid date format
echo -e "${YELLOW}Test 4: Invalid Date Format${NC}"
response=$(curl -s -w "\n%{http_code}" "$BASE_URL/api/colors?date=invalid-date")
http_code=$(echo "$response" | tail -n1)
body=$(echo "$response" | head -n-1)

if [ "$http_code" = "400" ]; then
    print_result 0 "API correctly returned 400 for invalid date"
    echo -e "   Response: $body\n"
else
    print_result 1 "API should return 400 for invalid date (got $http_code)"
fi

# Test 5: Wrong HTTP method
echo -e "${YELLOW}Test 5: Wrong HTTP Method (POST)${NC}"
response=$(curl -s -X POST -w "\n%{http_code}" "$BASE_URL/api/colors")
http_code=$(echo "$response" | tail -n1)

if [ "$http_code" = "405" ]; then
    print_result 0 "API correctly returned 405 for POST method"
    echo ""
else
    print_result 1 "API should return 405 for POST method (got $http_code)"
fi

# Test 6: Cache behavior (second request should be instant)
echo -e "${YELLOW}Test 6: Cache Behavior (Same Request Twice)${NC}"
today=$(date +%Y-%m-%d)

echo "   First request (may be slow)..."
start_time=$(date +%s%N)
response1=$(curl -s "$BASE_URL/api/colors?date=$today")
end_time=$(date +%s%N)
duration1=$(( (end_time - start_time) / 1000000 ))
from_cache1=$(echo "$response1" | jq -r '.from_cache')

echo "   Second request (should be instant)..."
start_time=$(date +%s%N)
response2=$(curl -s "$BASE_URL/api/colors?date=$today")
end_time=$(date +%s%N)
duration2=$(( (end_time - start_time) / 1000000 ))
from_cache2=$(echo "$response2" | jq -r '.from_cache')

echo -e "   First request:  ${duration1}ms (from_cache: $from_cache1)"
echo -e "   Second request: ${duration2}ms (from_cache: $from_cache2)"

if [ "$from_cache2" = "true" ]; then
    print_result 0 "Second request was served from cache"
else
    echo -e "${YELLOW}⚠${NC}  Second request not cached (might be first run)"
fi

if [ "$duration2" -lt "$duration1" ]; then
    print_result 0 "Second request was faster ($duration2ms vs $duration1ms)"
else
    echo -e "${YELLOW}⚠${NC}  Second request not significantly faster\n"
fi

echo ""

# Test 7: Future date (should fail)
echo -e "${YELLOW}Test 7: Future Date (Should Fail)${NC}"
future_date=$(date -d "tomorrow" +%Y-%m-%d 2>/dev/null || date -v+1d +%Y-%m-%d 2>/dev/null)
response=$(curl -s -w "\n%{http_code}" "$BASE_URL/api/colors?date=$future_date")
http_code=$(echo "$response" | tail -n1)
body=$(echo "$response" | head -n-1)

if [ "$http_code" = "500" ]; then
    print_result 0 "API correctly rejected future date"
    echo -e "   Response: $body\n"
else
    echo -e "${YELLOW}⚠${NC}  Expected 500 error for future date (got $http_code)\n"
fi

# Summary
echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo -e "${GREEN}All critical tests passed!${NC}"
echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo ""
echo "Notes:"
echo "  • Make sure OPENROUTER_API_KEY is set for full functionality"
echo "  • First requests may take 5-30 seconds (AI analysis)"
echo "  • Cached requests should be instant (<10ms)"
echo "  • Check server logs for detailed information"
echo ""
