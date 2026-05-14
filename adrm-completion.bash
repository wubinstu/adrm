# adrm bash/zsh completion
#
# This file is sourced by the auto-generated adrm-init.sh.
# Do NOT edit manually — it will be overwritten on reinstall.
#
# If you use adrm without the installer, source it directly:
#   source "/path/to/adrm-completion.bash"

# --- Bash completion ---
if [ -n "${BASH_VERSION:-}" ]; then
    _adrm_completions() {
        local cur prev prev2
        COMPREPLY=()
        cur="${COMP_WORDS[COMP_CWORD]}"
        prev="${COMP_WORDS[COMP_CWORD-1]}"
        if [ "$COMP_CWORD" -ge 2 ]; then
            prev2="${COMP_WORDS[COMP_CWORD-2]}"
        else
            prev2=""
        fi

        # Global long options that take no argument
        local global_opts="--help --version --default --reset-db --clear --query --query-all --restore --restore-all --clean --clean-all --recursive --color --deadline --sort-asc --sort-des --id --items --fname --fdate --fsize --rdate --cdate --state"

        # If previous word expects a value, provide completions
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
                # Suggest a date template
                COMPREPLY=($(compgen -W '"YYYY-MM-DD" "YYYY/MM/DD" "YYYY_MM_DD"' -- "$cur"))
                return 0
                ;;
            --id|--items|--fname|--fdate|--fsize|--rdate|--cdate)
                # These need user-supplied values, no generic completion
                return 0
                ;;
        esac

        # Otherwise complete with options or filenames
        COMPREPLY=($(compgen -W "${global_opts}" -- "$cur"))
        _filedir 2>/dev/null || compgen -f -- "$cur" >/dev/null
    }
    complete -F _adrm_completions adrm
fi

# --- Zsh completion ---
if [ -n "${ZSH_VERSION:-}" ]; then
    # Register for both 'adrm' and 'rm' (when aliased)
    _adrm_zsh() {
        local -a commands opts filters sort_fields state_vals

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

    compdef _adrm_zsh adrm
fi
