# Python version manager
export PYENV_ROOT="$HOME/.pyenv"
export PATH="$PYENV_ROOT/bin:$PATH"
eval "$(pyenv init --path)"
eval "$(pyenv init -)"

# local env
[ -f "$HOME/.local/bin/env" ] && source "$HOME/.local/bin/env"