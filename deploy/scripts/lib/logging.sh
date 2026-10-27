#!/usr/bin/env bash
# Linux Server Manager - Logging Library

# Cores ANSI
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
MAGENTA='\033[0;35m'
CYAN='\033[0;36m'
BOLD='\033[1m'
NC='\033[0m' # No Color

# Suprime cores se saída não for terminal interativo
if [[ ! -t 1 ]]; then
    RED=''
    GREEN=''
    YELLOW=''
    BLUE=''
    MAGENTA=''
    CYAN=''
    BOLD=''
    NC=''
fi

log_info() {
    echo -e "${BLUE}[INFO]${NC} $*"
}

log_success() {
    echo -e "${GREEN}[OK]${NC} $*"
}

log_warn() {
    echo -e "${YELLOW}[AVISO]${NC} $*" >&2
}

log_error() {
    echo -e "${RED}[ERRO]${NC} $*" >&2
}

log_dry() {
    echo -e "${MAGENTA}[DRY-RUN]${NC} $*"
}

log_step() {
    echo -e "\n${BOLD}${CYAN}==>${NC} ${BOLD}$*${NC}"
}
