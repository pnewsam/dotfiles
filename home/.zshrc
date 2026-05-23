# Oh My Zsh
export ZSH="$HOME/.oh-my-zsh"
ZSH_THEME="robbyrussell"
plugins=(git zsh-autosuggestions zsh-syntax-highlighting)
source "$ZSH/oh-my-zsh.sh"

# Completion system
autoload -U compinit && compinit -u

# Load feature modules
for module in ~/.config/zsh/*.zsh; do
  [ -f "$module" ] && source "$module"
done