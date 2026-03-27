#!/usr/bin/env bash
printf "Continue? (y/n) "
read -r answer
if [ "$answer" = "y" ]; then
  echo "confirmed"
  exit 0
fi
echo "cancelled"
exit 1
