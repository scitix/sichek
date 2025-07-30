#!/bin/bash

color_green="\033[1;32m"
color_yellow="\033[1;33m"
color_purple="\033[1;35m"
color_reset="\033[0m"

echo_back() {
    local _cmdLog=${1}
    printf "[${color_purple}EXEC${color_reset}] ${_cmdLog}\n"
    eval ${_cmdLog}
}

echo_info() {
    local _cmdLog=${1}
    printf "[${color_green}INFO${color_reset}] ${_cmdLog}\n"
}

echo_warn() {
    local _cmdLog=${1}
    printf "[${color_yellow}WARN${color_reset}] ${_cmdLog}\n"
}
