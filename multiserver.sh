#!/bin/bash
#
# multiserver.sh - A script to manage multiple server instances
# Version: 1.0.0
#
# This script allows you to start multiple instances of a server application,
# check their status, and stop them after a specified timeout.
#

# Function to display help message
show_help() {
  echo "Usage: $0 [OPTIONS] <number-of-instances>"
  echo
  echo "Start multiple instances of the server application."
  echo
  echo "Options:"
  echo "  -h, --help              Display this help message and exit"
  echo "  -v, --verbose           Enable verbose output"
  echo "  -c, --command COMMAND   Specify a custom command to run (default: go run ./cmd/server)"
  echo "  -t, --timeout SECONDS   Automatically stop servers after specified seconds"
  echo "  -s, --status            Check status of running servers"
  echo
  echo "Examples:"
  echo "  $0 3                    Start 3 server instances"
  echo "  $0 -v 5                 Start 5 server instances with verbose output"
  echo "  $0 -c \"go run ./cmd/custom\" 2   Start 2 instances with a custom command"
  echo "  $0 -t 60 3              Start 3 instances and stop them after 60 seconds"
}

# Default values
verbose=false
command="go run ./cmd/server"
timeout=0
check_status=false
pid_file="/tmp/multiserver_pids.txt"

# Parse command line options
while [[ $# -gt 0 ]]; do
  case "$1" in
    -h|--help)
      show_help
      exit 0
      ;;
    -v|--verbose)
      verbose=true
      shift
      ;;
    -c|--command)
      if [[ -z "$2" || "$2" == -* ]]; then
        echo "Error: --command requires an argument"
        exit 1
      fi
      command="$2"
      shift 2
      ;;
    -t|--timeout)
      if [[ -z "$2" || "$2" == -* ]]; then
        echo "Error: --timeout requires an argument"
        exit 1
      fi
      if ! [[ "$2" =~ ^[0-9]+$ ]]; then
        echo "Error: timeout must be a positive number"
        exit 1
      fi
      timeout="$2"
      shift 2
      ;;
    -s|--status)
      check_status=true
      shift
      ;;
    *)
      # If it's not an option, assume it's the number of instances
      if [[ $1 =~ ^[0-9]+$ ]]; then
        num_instances=$1
      else
        echo "Error: Invalid argument '$1'"
        show_help
        exit 1
      fi
      shift
      ;;
  esac
done

# Check if the number of instances was provided
if [ -z "$num_instances" ]; then
  echo "Error: Number of instances not specified"
  show_help
  exit 1
fi

# Validate number of instances
if [ "$num_instances" -lt 1 ]; then
  echo "Error: Number of instances must be at least 1"
  exit 1
fi

# Array to store process IDs and ports
declare -a pids

# Function to check status of running servers
check_server_status() {
  echo "Checking status of server instances..."
  local running_count=0
  local total_count=0

  # If the PID file exists, read PIDs from it
  if [ -f "$pid_file" ]; then
    # Read PIDs from file into an array
    mapfile -t file_pids < "$pid_file"

    for pid in "${file_pids[@]}"; do
      # Skip empty lines
      if [ -z "$pid" ]; then
        continue
      fi

      total_count=$((total_count+1))

      if kill -0 "$pid" 2>/dev/null; then
        echo "Server with PID $pid is running"
        running_count=$((running_count+1))
      else
        echo "Server with PID $pid is not running"
      fi
    done

    echo "$running_count out of $total_count servers are running"
  else
    echo "No PID file found at $pid_file"
    echo "No server instances appear to be running"
  fi
}

# Function to kill all processes when Ctrl+C is pressed or script exits
cleanup() {
  echo
  echo "Terminating all server instances..."

  # If the PID file exists, read PIDs from it
  if [ -f "$pid_file" ]; then
    mapfile -t file_pids < "$pid_file"

    for pid in "${file_pids[@]}"; do
      # Skip empty lines
      if [ -z "$pid" ]; then
        continue
      fi

      if kill -0 "$pid" 2>/dev/null; then
        echo "Stopping server with PID $pid"
        kill -SIGTERM "$pid"
      fi
    done

    # Remove the PID file
    rm -f "$pid_file"
    echo "Removed PID file: $pid_file"
  else
    # If no PID file exists, use the pids array
    for pid in "${pids[@]}"; do
      if kill -0 "$pid" 2>/dev/null; then
        echo "Stopping server with PID $pid"
        kill -SIGTERM "$pid"
      fi
    done
  fi

  echo "All servers stopped"
  exit
}

# Setup traps for various signals
trap 'cleanup' SIGINT SIGTERM EXIT

# If status check is requested, only check status and exit
if [ "$check_status" = true ]; then
  check_server_status
  exit 0
fi

echo "Starting $num_instances server instance(s)..."
echo "Using command: $command"

# Clear the PID file if it exists
> "$pid_file"

# Start the specified number of instances of the program in the background
for (( i=0; i<num_instances; i++ )); do
  if [ "$verbose" = true ]; then
    echo "Starting server instance #$((i+1))..."
  fi

  # Use eval to properly handle quoted arguments in the command
  eval "$command" &
  current_pid=$!
  pids+=($current_pid)

  # Append the PID to the PID file
  echo "$current_pid" >> "$pid_file"

  if [ "$verbose" = true ]; then
    echo "Server instance #$((i+1)) started with PID $current_pid"
  fi
done

echo "All $num_instances server instance(s) started successfully"
echo "PIDs saved to $pid_file"
if [ "$verbose" = true ]; then
  echo "Server PIDs: ${pids[*]}"
fi

# If timeout is specified, set up a timer to stop servers
if [ "$timeout" -gt 0 ]; then
  echo "Servers will automatically stop after $timeout seconds"

  # Start a background process to handle the timeout
  (
    sleep "$timeout"
    echo "Timeout of $timeout seconds reached"
    # Send SIGTERM to the parent process
    kill -SIGTERM $$
  ) &
  timeout_pid=$!

  # Add the timeout process to the list of PIDs to clean up
  pids+=($timeout_pid)
fi

echo "Press Ctrl+C to stop all servers"

# Wait for all background processes to finish
wait
