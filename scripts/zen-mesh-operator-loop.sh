#!/bin/bash
# Zen-Mesh Operational Loop (24/7)
# Continuously discovers, analyzes, and processes Zen-Mesh Jira tickets

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ZEN_BRAIN_DIR="$(dirname "$SCRIPT_DIR/..")"
export ZEN_BRAIN_OFFICE_ALLOW_STUB_KB=1
export ZEN_BRAIN_OFFICE_ALLOW_STUB_LEDGER=1
export JIRA_URL=https://zen-mesh.atlassian.net
export JIRA_EMAIL=zen@zen-mesh.io
export JIRA_PROJECT_KEY=ZB

echo "=== Zen-Mesh Operational Loop ==="
echo "Started: $(date -Iseconds)"
echo "Zen-Brain: $ZEN_BRAIN_DIR/bin/zen-brain"
echo ""

# Function: Discover candidate tickets
discover_tickets() {
	local search_jql='status IN ("To Do", "In Progress", "Backlog") AND assignee IS EMPTY AND priority IN ("High", "Medium")'
	echo "[1/4] Discovering candidate tickets..."
	echo "  JQL: $search_jql"
	echo ""

	local result=$($ZEN_BRAIN_DIR/bin/zen-brain office search "$search_jql" 2>&1)
	echo "  Found: $(echo "$result" | grep -oP 'Found [0-9]+ item' || echo '0 items')"
	echo ""
}

# Function: Analyze ticket
analyze_ticket() {
	local ticket_key="$1"
	echo "[2/4] Analyzing ticket: $ticket_key"
	echo "  Fetching details..."
	$ZEN_BRAIN_DIR/bin/zen-brain office fetch "$ticket_key" > /tmp/zen-brain-ticket-$ticket_key.json 2>&1
	echo ""

	if [ $? -eq 0 ]; then
		echo "  Analyzing with Ollama..."
		$ZEN_BRAIN_DIR/bin/zen-brain analyze work-item "$ticket_key" > /tmp/zen-brain-analysis-$ticket_key.txt 2>&1
		echo ""

		echo "  ✓ Analysis complete"
		echo "  Output: /tmp/zen-brain-analysis-$ticket_key.txt"
	else
		echo "  ✗ Failed to fetch/analyze ticket"
	fi
	echo ""
}

# Function: Generate recommendation
generate_recommendation() {
	local ticket_key="$1"
	echo "[3/4] Generating recommendation for: $ticket_key"

	echo "  Action Class Analysis:"
	echo "    - Class A (Always Allowed): fetch, analyze, summarize, classify, recommend"
	echo "    - Class B (Safe Write-Back): Jira comments, artifact attachments, safe status updates"
	echo "    - Class C (Approval Required): repo writes, merges, deploys, meaningful status transitions"
	echo ""
}

# Function: Interactive menu
show_menu() {
	echo "=== Operational Menu ==="
	echo "1. Discover candidate tickets"
	echo "2. Analyze specific ticket"
	echo "3. Generate recommendation"
	echo "4. Run continuous loop (auto)"
	echo "5. Health checks"
	echo "6. Exit"
	echo ""
	read -p "Select option [1-6]: " choice

	case $choice in
		1)
			discover_tickets
			;;
		2)
			read -p "Enter ticket key (e.g., ZB-XXX): " ticket_key
			analyze_ticket "$ticket_key"
			generate_recommendation "$ticket_key"
			;;
		3)
			read -p "Enter ticket key (e.g., ZB-XXX): " ticket_key
			generate_recommendation "$ticket_key"
			;;
		4)
			echo "[4/4] Running continuous loop..."
			echo "  Discovering tickets..."
			discover_tickets

			echo "  Analyzing tickets..."
			# TODO: Parse search results and analyze each ticket
			# For now, analyze first ticket interactively
			;;
		5)
			echo "[5/5] Running health checks..."
			echo ""

			echo "  1. Office Doctor"
			$ZEN_BRAIN_DIR/bin/zen-brain office doctor
			echo ""

			echo "  2. Runtime Doctor"
			$ZEN_BRAIN_DIR/bin/zen-brain runtime doctor
			echo ""

			echo "  3. Llama.cpp L1 Health"
			curl -s http://127.0.0.1:56227/v1/models
			if [ $? -eq 0 ]; then
				echo "  ✓ llama.cpp L1 responding"
			else
				echo "  ✗ llama.cpp L1 not responding"
			fi
			echo ""

			echo "  4. Jira Connectivity"
			curl -s -u "$JIRA_EMAIL:$JIRA_TOKEN" \
				-H "Accept: application/json" \
				"https://zen-mesh.atlassian.net/rest/api/3/myself" > /dev/null 2>&1
			if [ $? -eq 0 ]; then
				echo "  ✓ Jira connectivity OK"
			else
				echo "  ✗ Jira connectivity FAILED"
			fi
			echo ""
			;;
		6)
			echo "Exiting..."
			exit 0
			;;
		*)
			echo "Invalid option: $choice"
			;;
	esac
}

# Function: Continuous loop (non-interactive)
run_continuous_loop() {
	local max_tickets=3
	local interval_seconds=3600  # 1 hour

	echo "[4/4] Continuous mode (max $max_tickets tickets, every $interval_seconds seconds)"
	echo "Press Ctrl+C to stop"
	echo ""

	local count=0
	while true; do
		echo "--- Loop iteration: $((count + 1)) ---"
		echo "Timestamp: $(date)"

		# Discover tickets
		discover_tickets

		# Increment counter
		count=$((count + 1))

		# Check max tickets
		if [ $count -ge $max_tickets ]; then
			echo "Reached max tickets ($max_tickets), stopping"
			break
		fi

		# Wait for next iteration
		echo "Waiting $interval_seconds seconds..."
		sleep "$interval_seconds"
	done
}

# Main
if [ "$1" == "--continuous" ]; then
	run_continuous_loop
else
	show_menu
fi
