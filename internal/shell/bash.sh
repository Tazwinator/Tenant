# tenant: the bash hook (bash 5.1 or newer).
#
# This is everything tenant does inside your shell. It uses only bash
# builtins. It never evals anything tenant prints: tenant answers with a verb
# and integers, and this script decides what to do with them. It does
# nothing at all until you run `tenant start`, and nothing as root or with
# TENANT_OFF set. `tenant evict` switches it off in every shell at the next
# prompt; then delete the eval line from ~/.bashrc.

if [[ $- == *i* && -z ${_tenant_loaded-} ]] &&
	((BASH_VERSINFO[0] > 5 || (BASH_VERSINFO[0] == 5 && BASH_VERSINFO[1] >= 1))); then

_tenant_loaded=1
_tenant_bin=__TENANT_BIN__
_tenant_dir=${XDG_STATE_HOME:-$HOME/.local/state}/tenant
[[ $_tenant_dir == /* ]] || _tenant_dir=$HOME/.local/state/tenant
_tenant_st=0     # status of the last command
_tenant_hist=    # HISTCMD at the last prompt
_tenant_cmd=     # first word of the last command, or empty
_tenant_shape=-  # what kind of command it was (ls:p, clear, ...)
_tenant_first=1  # 1 until the first prompt has been drawn
_tenant_live=0   # 1 once tenant has been active in this shell
_tenant_ps1=     # PS1 as it was before a one-prompt change
_tenant_ps1_mod= # PS1 as tenant changed it
_tenant_pwd=     # the altered directory for a prompt.glyph
_tenant_pwdb=    # ... and its last component
_tenant_rtime=   # the right-aligned text for a prompt.time
_tenant_up=      # readline function Up is bound to, if tenant can borrow it
_tenant_armed=   # 1 while the next Up press shows a ghost
_tenant_capv=0
export TENANT_HOOK=bash

# Runs first each prompt: keep $?, undo last prompt's change, and learn the
# first word of the command that just ran, from the in-memory history entry
# (never the history file). Commands you start with a space never get there
# if HISTCONTROL has ignorespace, so tenant never sees them.
_tenant_pre() {
	_tenant_st=$?
	if [[ -n $_tenant_ps1_mod && $PS1 == "$_tenant_ps1_mod" ]]; then
		PS1=$_tenant_ps1
	fi
	_tenant_ps1='' _tenant_ps1_mod=''
	_tenant_cmd='' _tenant_shape=-
	if [[ -e $_tenant_dir/active && $HISTCMD != "$_tenant_hist" ]]; then
		if ((_tenant_first)); then
			_tenant_hist=$HISTCMD
		else
			_tenant_hist=$HISTCMD
			local h
			h=$(HISTTIMEFORMAT='' builtin history 1)
			[[ $h =~ ^[[:space:]]*[0-9]+\*?[[:space:]]+(.*)$ ]] && _tenant_classify "${BASH_REMATCH[1]}"
		fi
	fi
	return "$_tenant_st"
}

# Keeps only the first word, and works out whether the command was an
# ls tenant could imitate. Nothing else about the command leaves the shell.
_tenant_classify() {
	local line=$1 w x rest a long='' color='' all='' bad=''
	line=${line#"${line%%[![:space:]]*}"}
	w=${line%%[[:space:]]*}
	rest=${line:${#w}}
	w=${w#\\}
	x=$w
	if [[ -n ${BASH_ALIASES[$w]+set} ]]; then
		line=${BASH_ALIASES[$w]}$rest
		line=${line#"${line%%[![:space:]]*}"}
		x=${line%%[[:space:]]*}
		rest=${line:${#x}}
	fi
	w=${w##*/} x=${x##*/}
	[[ $w =~ ^[A-Za-z0-9._+:@-]{1,40}$ ]] || return
	_tenant_cmd=$w
	case $x in
	ls | eza | exa)
		[[ $x != ls ]] && color=c
		local -
		set -f
		for a in $rest; do
			case $a in
			--color=never | --colour=never) color= ;;
			--color* | --colour*) color=c ;;
			--icons* | -*[!-A-Za-z0-9=,]*) bad=x ;;
			--long | --format=long | --format=verbose) long=l ;;
			--all | --almost-all) all=a ;;
			--*) ;;
			-*)
				[[ $a == *l* ]] && long=l
				[[ $a == *[aA]* ]] && all=a
				[[ $a == *[Rd]* ]] && bad=x
				;;
			*) bad=x ;;
			esac
		done
		if [[ -n $bad ]]; then _tenant_shape=ls:x; else _tenant_shape=ls:${long:-p}$color$all; fi
		;;
	clear) _tenant_shape=clear ;;
	cd) _tenant_shape='cd' ;;
	esac
}

# What this shell can show, as a bitmask (see internal/mech).
_tenant_caps() {
	local c=0 loc=${LC_ALL:-${LC_CTYPE:-${LANG-}}}
	if [[ ($PS1 == *'\w'* || $PS1 == *'\W'*) && -z ${PROMPT_DIRTRIM-} ]] && shopt -q promptvars; then
		((c |= 1))
	fi
	[[ -n $_tenant_up && -o emacs && -z ${BLE_VERSION-} ]] && ((c |= 2))
	[[ $loc == *[Uu][Tt][Ff]-8* || $loc == *[Uu][Tt][Ff]8* ]] && ((c |= 4))
	case ${TERM-} in
	xterm* | rxvt* | alacritty* | foot* | kitty* | wezterm* | st-* | konsole* | gnome* | vte* | contour* | ghostty*) ((c |= 8)) ;;
	esac
	[[ $PS1 != *$'\n'* && $PS1 != *'\n'* ]] && shopt -q promptvars && ((c |= 16))
	declare -F _tenant_cnf >/dev/null && ((c |= 32))
	_tenant_capv=$c
}

# Runs last each prompt, after any prompt framework has built PS1.
_tenant_post() {
	local st=$_tenant_st
	[[ -n $_tenant_armed ]] && _tenant_disarm
	if [[ ! -e $_tenant_dir/active ]]; then
		if ((_tenant_live)); then
			_tenant_unload
			return "$st"
		fi
		_tenant_first=0
		return "$st"
	fi
	if [[ -n ${TENANT_OFF-} ]] || ((EUID == 0)); then
		_tenant_first=0
		return "$st"
	fi
	_tenant_live=1
	_tenant_caps
	local out verb a b c
	out=$("$_tenant_bin" _hook prompt --shell bash --status "$st" --cmd "$_tenant_cmd" \
		--shape "$_tenant_shape" --first "$_tenant_first" --cols "${COLUMNS:-80}" --caps "$_tenant_capv")
	if (($? == 126 || $? == 127)); then
		_tenant_unload # the binary has gone: stop quietly
		return "$st"
	fi
	_tenant_first=0
	while read -r verb a b c; do
		case $verb in
		glyph) _tenant_glyph "$a" "$b" "$c" ;;
		time) _tenant_time ;;
		ghost) _tenant_arm ;;
		esac
	done <<<"$out"
	return "$st"
}

# prompt.glyph: show the directory with one letter changed, for one prompt.
# Arguments are integers: position from the end, the letter there, and the
# letter to show instead. PS1 gets a reference to a variable, so the path
# itself never goes through prompt expansion.
_tenant_glyph() {
	[[ $1 =~ ^[0-9]+$ && $2 =~ ^[0-9]+$ && $3 =~ ^[0-9]+$ ]] || return
	(($2 >= 97 && $2 <= 122 && $3 >= 97 && $3 <= 122)) || return
	local p=$PWD f t i hex
	[[ $p == "$HOME" || $p == "$HOME"/* ]] && p=\~${p#"$HOME"}
	i=$((${#p} - $1))
	((i > 0)) || return
	# The format is built from a validated integer, so it is safe here.
	printf -v hex '%x' "$2"
	# shellcheck disable=SC2059
	printf -v f "\\x$hex"
	printf -v hex '%x' "$3"
	# shellcheck disable=SC2059
	printf -v t "\\x$hex"
	[[ ${p:i:1} == "$f" ]] || return
	_tenant_pwd=${p:0:i}$t${p:i+1}
	_tenant_pwdb=${_tenant_pwd##*/}
	_tenant_ps1=$PS1
	# shellcheck disable=SC2016 # a literal reference, expanded by bash at prompt time
	PS1=${PS1//'\w'/'${_tenant_pwd}'}
	# shellcheck disable=SC2016
	PS1=${PS1//'\W'/'${_tenant_pwdb}'}
	_tenant_ps1_mod=$PS1
}

# prompt.time: a right-aligned time for one prompt. It is drawn inside
# \[ \] so bash counts it as zero width, then the cursor returns.
_tenant_time() {
	local t
	t=$("$_tenant_bin" _text time) || return
	[[ -n $t && ${#t} -le 20 && $t != *[[:cntrl:]]* ]] || return
	printf -v _tenant_rtime '%*s\r' "${COLUMNS:-80}" "$t"
	_tenant_ps1=$PS1
	PS1='\[${_tenant_rtime}\]'$PS1
	_tenant_ps1_mod=$PS1
}

# history.ghost: borrow Up for one press. The line is filled in the editor
# only; it never goes into the history list or the history file.
_tenant_arm() {
	[[ -n $_tenant_up ]] || return
	bind -x '"\e[A": _tenant_ghost' 2>/dev/null
	bind -x '"\eOA": _tenant_ghost' 2>/dev/null
	_tenant_armed=1
}

_tenant_disarm() {
	bind "\"\\e[A\": $_tenant_up" 2>/dev/null
	bind "\"\\eOA\": $_tenant_up" 2>/dev/null
	_tenant_armed=
}

_tenant_ghost() {
	local g
	g=$("$_tenant_bin" _ghost 2>/dev/null)
	_tenant_disarm
	[[ -n $g && $g != *[[:cntrl:]]* ]] || return
	READLINE_LINE=$g
	READLINE_POINT=${#g}
}

# Which history function Up runs, so it can be put back. Up keys owned by
# other tools (atuin, mcfly, fzf) are left alone.
_tenant_probe_up() {
	local f
	for f in previous-history history-search-backward history-substring-search-backward; do
		if [[ $(bind -q "$f" 2>/dev/null) == *'"\e[A"'* ]]; then
			_tenant_up=$f
			break
		fi
	done
	[[ $(bind -X 2>/dev/null) == *'\e[A'* ]] && _tenant_up=
}

# notfound.remark: wrap any existing handler (pkgfile, for example), let it
# run first, unchanged, then let tenant add a line.
if declare -F command_not_found_handle >/dev/null; then
	_tenant_fn=$(declare -f command_not_found_handle)
	eval "_tenant_cnf_orig${_tenant_fn#command_not_found_handle}"
	unset _tenant_fn
fi

_tenant_cnf() {
	local rc=127
	if declare -F _tenant_cnf_orig >/dev/null; then
		_tenant_cnf_orig "$@"
		rc=$?
	else
		printf 'bash: %s: command not found\n' "$1" >&2
	fi
	if [[ -e $_tenant_dir/active && -z ${TENANT_OFF-} ]] && ((EUID != 0)); then
		# Only if the line made it into history: a command started with a
		# space (with ignorespace) stays invisible here too.
		local w=${1##*/} h
		h=$(HISTTIMEFORMAT='' builtin history 1)
		[[ $h =~ ^[[:space:]]*[0-9]+\*?[[:space:]]+(.*)$ && " ${BASH_REMATCH[1]}" == *[[:space:]]"$1"* ]] || return "$rc"
		[[ $w =~ ^[A-Za-z0-9._+:@-]{1,40}$ ]] &&
			"$_tenant_bin" _hook notfound --shell bash --cmd "$w" --caps "$_tenant_capv" >/dev/null
	fi
	return "$rc"
}

command_not_found_handle() { _tenant_cnf "$@"; }

# Put everything back the way it was.
_tenant_unload() {
	[[ -n $_tenant_armed ]] && _tenant_disarm
	if [[ -n $_tenant_ps1_mod && $PS1 == "$_tenant_ps1_mod" ]]; then
		PS1=$_tenant_ps1
	fi
	if declare -F _tenant_cnf_orig >/dev/null; then
		local fn
		fn=$(declare -f _tenant_cnf_orig)
		eval "command_not_found_handle${fn#_tenant_cnf_orig}"
	else
		unset -f command_not_found_handle
	fi
	local e
	local -a keep=()
	for e in "${PROMPT_COMMAND[@]}"; do
		[[ $e == _tenant_pre || $e == _tenant_post ]] || keep+=("$e")
	done
	PROMPT_COMMAND=("${keep[@]}")
	unset -f _tenant_pre _tenant_classify _tenant_caps _tenant_post _tenant_glyph \
		_tenant_time _tenant_arm _tenant_disarm _tenant_ghost _tenant_probe_up \
		_tenant_cnf _tenant_cnf_orig _tenant_unload
	unset _tenant_bin _tenant_dir _tenant_st _tenant_hist _tenant_cmd _tenant_shape \
		_tenant_first _tenant_live _tenant_ps1 _tenant_ps1_mod _tenant_pwd _tenant_pwdb \
		_tenant_rtime _tenant_up _tenant_armed _tenant_capv _tenant_loaded TENANT_HOOK
}

_tenant_probe_up
PROMPT_COMMAND=(_tenant_pre ${PROMPT_COMMAND[@]+"${PROMPT_COMMAND[@]}"} _tenant_post)

fi
