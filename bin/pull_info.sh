#!/usr/bin/env bash
# Usage: ./pull_info.sh [--yes]

yes_all=false
for arg in "$@"; do
  if [[ "$arg" == "--yes" ]]; then
    yes_all=true
  fi
done

prompt_run() {
  local cmd="$1"

  if [[ "$yes_all" == true ]]; then
    echo "auto-yes: $cmd"
    eval "$cmd"
    return
  fi

  while true; do
    printf 'Run "%s"? (yes/no): ' "$cmd"
    read -r answer
    case "$answer" in
      yes|y)
        eval "$cmd"
        break
        ;;
      no|n)
        echo "skip: $cmd"
        break
        ;;
      *)
        echo "please answer yes or no."
        ;;
    esac
  done
}

commands=(
  "ghub-desk pull --users"
  "ghub-desk pull --detail-users --interval-time=5s"
  "ghub-desk pull --outside-users --interval-time=5s"
  "ghub-desk pull --teams --interval-time=5s"
  "ghub-desk pull --repos --interval-time=5s"
  "ghub-desk pull --all-repos-users --interval-time=5s"
  "ghub-desk pull --all-repos-teams --interval-time=5s"
  "ghub-desk pull --all-teams-users --interval-time=5s"
)

for cmd in "${commands[@]}"; do
  prompt_run "$cmd"
done
