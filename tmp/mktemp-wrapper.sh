#!/bin/bash
STABLE_TEMP_DIR="/Users/xiaodian/IdeaProjects/higress/tmp/higress-istio-build"
HIGRESS_TEMP_DIR="/Users/xiaodian/IdeaProjects/higress/tmp/higress-build"

if [[ "$1" == "-d" ]]; then
    if [[ "$2" == "-u" ]]; then
        # Return path without creating directory
        dir_name=$(openssl rand -hex 8 2>/dev/null || date +%s%N | tail -c 8)
        echo "$STABLE_TEMP_DIR/tmp_$dir_name"
    else
        # Create directory in our stable temp location
        dir_name=$(openssl rand -hex 8 2>/dev/null || date +%s%N | tail -c 8)
        dir_path="$STABLE_TEMP_DIR/tmp_$dir_name"
        mkdir -p "$dir_path"
        echo "$dir_path"
    fi
else
    # Create file in our stable temp location
    file_name=$(openssl rand -hex 8 2>/dev/null || date +%s%N | tail -c 8)
    file_path="$HIGRESS_TEMP_DIR/tmp_$file_name"
    touch "$file_path"
    echo "$file_path"
fi
