# adrm bash/zsh completion
#
# This file is sourced by the auto-generated adrm-init.sh.
# Do NOT edit manually — it will be overwritten on reinstall.
#
# If you use adrm without the installer, source it directly:
#   source "/path/to/adrm-completion.sh"

# --- Bash completion ---
if [ -n "${BASH_VERSION:-}" ]; then
    _adrm_completions() {
        local cur prev
        COMPREPLY=()
        cur="${COMP_WORDS[COMP_CWORD]}"
        prev="${COMP_WORDS[COMP_CWORD-1]}"

        # When /bin/rm is invoked by full path, fall back to default completion
        local cmd="${COMP_WORDS[0]}"
        if [[ "$cmd" == */* ]]; then
            local real_path
            real_path=$(readlink -f "$cmd" 2>/dev/null || echo "$cmd")
            if [[ "$real_path" == */bin/rm ]]; then
                compopt -o default
                COMPREPLY=()
                return 0
            fi
        fi

        # Arguments that expect a value — provide contextual completions
        case "$prev" in
            --sort-asc|--sort-des)
                COMPREPLY=($(compgen -W "id fname fdate fsize rdate cdate state" -- "$cur"))
                return 0
                ;;
            --state)
                COMPREPLY=($(compgen -W "recycled restored cleaned exception" -- "$cur"))
                return 0
                ;;
            --deadline)
                COMPREPLY=($(compgen -W '"YYYY-MM-DD" "YYYY/MM/DD" "YYYY_MM_DD"' -- "$cur"))
                return 0
                ;;
            --id|--items|--fname|--fdate|--fsize|--rdate|--cdate)
                return 0
                ;;
        esac

        # When cursor starts with -, complete options only (short and long)
        if [[ "$cur" == -* ]]; then
            local opts="-f -i -I -r -R -d -v -- --help --version --default --reset-db --clear --query --query-all --restore --restore-all --clean --clean-all --recursive --color --deadline --sort-asc --sort-des --id --items --fname --fdate --fsize --rdate --cdate --state"
            COMPREPLY=($(compgen -W "$opts" -- "$cur"))
            return 0
        fi

        # Otherwise, complete filenames only
        if type _filedir &>/dev/null; then
            _filedir
        else
            COMPREPLY=($(compgen -f -- "$cur"))
        fi
    }

    # Save original rm completion for potential restore on uninstall
    _adrm_orig_rm_complete=$(complete -p rm 2>/dev/null || true)

    complete -F _adrm_completions adrm
    complete -F _adrm_completions rm
fi

# --- Zsh completion ---
if [ -n "${ZSH_VERSION:-}" ]; then
    _adrm_zsh() {
        local -a commands opts filters sort_fields state_vals

        # When /bin/rm is invoked by full path, fall back to default completion
        if [[ $words[1] == */* ]]; then
            local real_path
            real_path=$(readlink -f "$words[1]" 2>/dev/null || echo "$words[1]")
            if [[ "$real_path" == */bin/rm ]]; then
                _default
                return
            fi
        fi

        commands=(
            '--query:show recent records (default 10)'
            '--query-all:show all records'
            '--restore:restore the most recent recycled file'
            '--restore-all:restore all recycled files'
            '--clean:permanently delete the most recent recycled file'
            '--clean-all:permanently delete all recycled files'
            '--clear:clean all expired files'
            '--reset-db:reset the database'
            '--default:generate default configuration file'
            '--help:display help and exit'
            '--version:output version information and exit'
        )

        opts=(
            '-f[ignore nonexistent files, bypass ignore rules]'
            '-i[prompt before every removal]'
            '-I[prompt once before removing more than 3 files]'
            '-r[remove directories recursively]'
            '-R[remove directories recursively]'
            '-d[remove empty directories]'
            '-v[verbose output]'
            '--recursive[remove directories recursively]'
            '--color[colorize output]'
        )

        filters=(
            '--id:filter by record ID (N, +N, -N)'
            '--items:limit to N most recent records'
            '--fname:filter by filename pattern'
            '--fdate:filter by original file modification date'
            '--fsize:filter by original file size'
            '--rdate:filter by recycle date'
            '--cdate:filter by cleanup date'
            '--state:filter by status'
            '--deadline:set absolute cleanup deadline'
        )

        sort_fields=(id fname fdate fsize rdate cdate state)
        state_vals=(recycled restored cleaned exception)

        _arguments -C \
            $opts \
            $commands \
            $filters \
            '--sort-asc[sort ascending]:field:('"${(j: :)sort_fields}"')' \
            '--sort-des[sort descending]:field:('"${(j: :)sort_fields}"')' \
            '--state[filter by status]:status:('"${(j: :)state_vals}"')' \
            '*:file:_files'
    }

    compdef _adrm_zsh adrm rm
fi
