# Git aliases and helpers

alias gb="git branch --sort=-committerdate"

lazy() {
    if [ -z "$1" ]; then
        echo "Please provide a commit message"
        return 1
    fi
    git add .
    git commit -m "$1"
    git push origin HEAD
}

wip() {
    git add .
    git commit -m "wip"
}

gsave() {
    git add .
    date_str=$(date -u +"%Y-%m-%dT%H:%M:%SZ")
    git commit -m "save: $date_str"
    git push origin HEAD
}