# tenant: the bash hook (bash 5.1 or newer).
#
# This is everything tenant does inside your shell. It uses only bash
# builtins. It never evals anything tenant prints: tenant answers with a verb
# and integers, and this script decides what to do with them. It does
# nothing at all until you run `tenant start`, and nothing as root or with
# TENANT_OFF set. `tenant evict` switches it off in every shell at the next
# prompt; then delete the eval line from ~/.bashrc.
#
# Every function returns 0 or the status it was given, so `set -e` and
# `set -u` shells are safe. Sourcing this again (re-reading ~/.bashrc) keeps
# tenant's state and re-attaches it to PROMPT_COMMAND.

if [[ $- == *i* ]] && ((BASH_VERSINFO[0] > 5 || (BASH_VERSINFO[0] == 5 && BASH_VERSINFO[1] >= 1))); then

if [[ -z ${_tenant_loaded-} ]]; then
	_tenant_loaded=1
	_tenant_st=0       # status of the last command
	_tenant_hist=''    # HISTCMD at the end of the last prompt
	_tenant_cmd=''     # first word of the last command, or empty
	_tenant_shape=-    # what kind of command it was (ls:p, clear, ...)
	_tenant_first=1    # 1 until the first prompt has been drawn
	_tenant_live=0     # 1 once tenant has been active in this shell
	_tenant_tick=0     # 1 between _tenant_pre and _tenant_post
	_tenant_ps1=''     # PS1 as it was before a one-prompt change
	_tenant_ps1_mod='' # PS1 as tenant changed it
	_tenant_pwd=''     # the altered directory for a prompt.glyph
	_tenant_pwdb=''    # ... and its last component
	_tenant_rtime=''   # the right-aligned text for a prompt.time
	_tenant_up_csi=''  # readline function on \e[A, if tenant can put it back
	_tenant_up_ss3=''  # readline function on \eOA, likewise
	_tenant_armed=''   # 1 while the next Up press shows a ghost
	_tenant_capv=0
	_tenant_r=''
fi
_tenant_bin=__TENANT_BIN__
_tenant_dir=${XDG_STATE_HOME:-$HOME/.local/state}/tenant
[[ $_tenant_dir == /* ]] || _tenant_dir=$HOME/.local/state/tenant

# Runs first each prompt: keep $?, undo last prompt's change, and learn the
# first word of the command that just ran, from the in-memory history entry
# (never the history file). Commands you start with a space never get there
# if HISTCONTROL has ignorespace, so tenant never sees them.
_tenant_pre() {
	_tenant_st=$?
	_tenant_tick=1
	if [[ -n $_tenant_ps1_mod && $PS1 == "$_tenant_ps1_mod" ]]; then
		PS1=$_tenant_ps1
	fi
	_tenant_ps1='' _tenant_ps1_mod=''
	_tenant_cmd='' _tenant_shape=-
	if ((!_tenant_first)) && [[ -e $_tenant_dir/active && $HISTCMD != "$_tenant_hist" ]]; then
		local h
		h=$(HISTTIMEFORMAT='' builtin history 1)
		if [[ $h =~ ^[[:space:]]*[0-9]+\*?[[:space:]]+(.*)$ ]]; then
			_tenant_classify "${BASH_REMATCH[1]}"
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
	if [[ -n $w && -n ${BASH_ALIASES[$w]+set} ]]; then
		line=${BASH_ALIASES[$w]}$rest
		line=${line#"${line%%[![:space:]]*}"}
		x=${line%%[[:space:]]*}
		rest=${line:${#x}}
	fi
	w=${w##*/} x=${x##*/}
	[[ $w =~ ^[A-Za-z0-9._+:@-]{1,40}$ ]] || return 0
	_tenant_cmd=$w
	case $x in
	ls | eza | exa)
		[[ $x != ls ]] && color=c
		local -
		set -f
		for a in $rest; do
			case $a in
			--color=never | --colour=never) color='' ;;
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
	return 0
}

# What this shell can show, as a bitmask (see internal/mech).
_tenant_caps() {
	local c=0 loc=${LC_ALL:-${LC_CTYPE:-${LANG-}}}
	if [[ ($PS1 == *'\w'* || $PS1 == *'\W'*) && -z ${PROMPT_DIRTRIM-} ]] && shopt -q promptvars; then
		((c |= 1))
	fi
	if [[ -n $_tenant_up_csi$_tenant_up_ss3 && -o emacs && -z ${BLE_VERSION-} ]]; then
		((c |= 2))
	fi
	if [[ $loc == *[Uu][Tt][Ff]-8* || $loc == *[Uu][Tt][Ff]8* ]]; then
		((c |= 4))
	fi
	case ${TERM-} in
	xterm* | rxvt* | alacritty* | foot* | kitty* | wezterm* | st-* | konsole* | gnome* | vte* | contour* | ghostty*) ((c |= 8)) ;;
	esac
	if [[ $PS1 != *$'\n'* && $PS1 != *'\n'* ]] && shopt -q promptvars; then
		((c |= 16))
	fi
	if [[ $(declare -f command_not_found_handle 2>/dev/null) == *_tenant_cnf* ]]; then
		((c |= 32))
	fi
	_tenant_capv=$c
	return 0
}

# Runs last each prompt, after any prompt framework has built PS1.
_tenant_post() {
	local st=$_tenant_st
	if ((!_tenant_tick)); then
		return "$st" # already ran for this prompt
	fi
	_tenant_tick=0
	if [[ -n $_tenant_armed ]]; then
		_tenant_disarm
	fi
	if [[ ! -e $_tenant_dir/active ]]; then
		if ((_tenant_live)); then
			_tenant_unload
			return "$st"
		fi
		_tenant_first=0 _tenant_hist=$HISTCMD
		return "$st"
	fi
	if [[ -n ${TENANT_OFF-} ]] || ((EUID == 0)); then
		_tenant_first=0 _tenant_hist=$HISTCMD
		return "$st"
	fi
	_tenant_live=1
	_tenant_caps
	local out verb a b c rc
	out=$("$_tenant_bin" _hook prompt --shell bash --pid "$$" --status "$st" --cmd "$_tenant_cmd" \
		--shape "$_tenant_shape" --first "$_tenant_first" --cols "${COLUMNS:-80}" --caps "$_tenant_capv")
	rc=$?
	if ((rc == 126 || rc == 127)); then
		_tenant_unload # the binary has gone: stop quietly
		return "$st"
	fi
	# Recorded after every other PROMPT_COMMAND has run, so lines that
	# `history -n` pulls in from other shells never count as yours.
	_tenant_first=0 _tenant_hist=$HISTCMD
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
	[[ $1 =~ ^[0-9]+$ && $2 =~ ^[0-9]+$ && $3 =~ ^[0-9]+$ ]] || return 0
	(($2 >= 97 && $2 <= 122 && $3 >= 97 && $3 <= 122)) || return 0
	local p=$PWD f t i hex
	if [[ $p == "$HOME" || $p == "$HOME"/* ]]; then
		p=\~${p#"$HOME"}
	fi
	i=$((${#p} - $1))
	((i > 0)) || return 0
	# The format is built from a validated integer, so it is safe here.
	printf -v hex '%x' "$2"
	# shellcheck disable=SC2059
	printf -v f "\\x$hex"
	printf -v hex '%x' "$3"
	# shellcheck disable=SC2059
	printf -v t "\\x$hex"
	[[ ${p:i:1} == "$f" ]] || return 0
	_tenant_pwd=${p:0:i}$t${p:i+1}
	_tenant_pwdb=${_tenant_pwd##*/}
	_tenant_ps1=$PS1
	# shellcheck disable=SC2016 # a literal reference, expanded by bash at prompt time
	PS1=${PS1//'\w'/'${_tenant_pwd}'}
	# shellcheck disable=SC2016
	PS1=${PS1//'\W'/'${_tenant_pwdb}'}
	_tenant_ps1_mod=$PS1
	return 0
}

# prompt.time: a right-aligned time for one prompt. It is drawn inside
# \[ \] so bash counts it as zero width, then the cursor returns.
_tenant_time() {
	local t
	t=$("$_tenant_bin" _text time --pid "$$") || return 0
	[[ -n $t && ${#t} -le 20 && $t != *[[:cntrl:]]* ]] || return 0
	printf -v _tenant_rtime '%*s\r' "${COLUMNS:-80}" "$t"
	_tenant_ps1=$PS1
	PS1='\[${_tenant_rtime}\]'$PS1
	_tenant_ps1_mod=$PS1
	return 0
}

# history.ghost: borrow Up for one press. The line is filled in the editor
# only; it never goes into the history list or the history file. Each Up key
# is only borrowed if tenant knows what to put back.
_tenant_arm() {
	if [[ -n $_tenant_up_csi ]]; then
		bind -x '"\e[A": _tenant_ghost' 2>/dev/null
	fi
	if [[ -n $_tenant_up_ss3 ]]; then
		bind -x '"\eOA": _tenant_ghost' 2>/dev/null
	fi
	if [[ -n $_tenant_up_csi$_tenant_up_ss3 ]]; then
		_tenant_armed=1
	fi
	return 0
}

_tenant_disarm() {
	if [[ -n $_tenant_up_csi ]]; then
		bind "\"\\e[A\": $_tenant_up_csi" 2>/dev/null
	fi
	if [[ -n $_tenant_up_ss3 ]]; then
		bind "\"\\eOA\": $_tenant_up_ss3" 2>/dev/null
	fi
	_tenant_armed=''
	return 0
}

_tenant_ghost() {
	local g h
	g=$("$_tenant_bin" _ghost --pid "$$" 2>/dev/null)
	_tenant_disarm
	if [[ -z $g || $g == *[[:cntrl:]]* ]]; then
		# Nothing to show after all: behave like a plain Up.
		g=''
		h=$(HISTTIMEFORMAT='' builtin history 1)
		if [[ $h =~ ^[[:space:]]*[0-9]+\*?[[:space:]]+(.*)$ && ${BASH_REMATCH[1]} != *[[:cntrl:]]* ]]; then
			g=${BASH_REMATCH[1]}
		fi
	fi
	READLINE_LINE=$g
	READLINE_POINT=${#g}
	return 0
}

# Which history function a key runs, in _tenant_r. readline may show Escape
# as \e or \M- depending on the locale.
_tenant_up_fn() {
	local f q
	_tenant_r=''
	for f in previous-history history-search-backward history-substring-search-backward; do
		q=$(bind -q "$f" 2>/dev/null)
		if [[ $q == *"\"\\e$1\""* || $q == *"\"\\M-$1\""* ]]; then
			_tenant_r=$f
			return 0
		fi
	done
	return 0
}

# Up keys owned by other tools (atuin, mcfly, fzf) through bind -x are left
# alone.
_tenant_probe_up() {
	local x
	x=$(bind -X 2>/dev/null)
	_tenant_up_fn '[A'
	_tenant_up_csi=$_tenant_r
	_tenant_up_fn 'OA'
	_tenant_up_ss3=$_tenant_r
	if [[ $x == *'\e[A'* || $x == *'\M-[A'* ]]; then
		_tenant_up_csi=''
	fi
	if [[ $x == *'\eOA'* || $x == *'\M-OA'* ]]; then
		_tenant_up_ss3=''
	fi
	return 0
}

# notfound.remark: wrap any existing handler (pkgfile, for example), let it
# run first, unchanged, then let tenant add a line.
if declare -F command_not_found_handle >/dev/null; then
	_tenant_fn=$(declare -f command_not_found_handle)
	if [[ $_tenant_fn != *_tenant_cnf* ]]; then
		eval "_tenant_cnf_orig${_tenant_fn#command_not_found_handle}"
	fi
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
	if [[ -e $_tenant_dir/active && -z ${TENANT_OFF-} && ${TENANT_HOOK-} == bash ]] && ((EUID != 0)); then
		# Only if the line made it into history: a command started with a
		# space (with ignorespace) stays invisible here too.
		local w=${1##*/} h
		h=$(HISTTIMEFORMAT='' builtin history 1)
		if [[ $h =~ ^[[:space:]]*[0-9]+\*?[[:space:]]+(.*)$ && " ${BASH_REMATCH[1]}" == *[[:space:]]"$1"* &&
			$w =~ ^[A-Za-z0-9._+:@-]{1,40}$ ]]; then
			"$_tenant_bin" _hook notfound --shell bash --pid "$$" --cmd "$w" --caps "$_tenant_capv" >/dev/null
		fi
	fi
	return "$rc"
}

command_not_found_handle() { _tenant_cnf "$@"; }

# Put everything back the way it was. Only undoes what is still tenant's:
# a not-found handler defined later is left alone.
_tenant_unload() {
	if [[ -n $_tenant_armed ]]; then
		_tenant_disarm
	fi
	if [[ -n $_tenant_ps1_mod && $PS1 == "$_tenant_ps1_mod" ]]; then
		PS1=$_tenant_ps1
	fi
	if [[ $(declare -f command_not_found_handle 2>/dev/null) == *_tenant_cnf* ]]; then
		if declare -F _tenant_cnf_orig >/dev/null; then
			local fn
			fn=$(declare -f _tenant_cnf_orig)
			eval "command_not_found_handle${fn#_tenant_cnf_orig}"
		else
			unset -f command_not_found_handle
		fi
	fi
	local e
	local -a keep=()
	for e in ${PROMPT_COMMAND[@]+"${PROMPT_COMMAND[@]}"}; do
		[[ $e == _tenant_pre || $e == _tenant_post ]] || keep+=("$e")
	done
	PROMPT_COMMAND=(${keep[@]+"${keep[@]}"})
	# Other tools (bash-preexec) may have folded these two into their own
	# PROMPT_COMMAND entries. Leave harmless stand-ins that keep $?.
	# shellcheck disable=SC2317 # called from PROMPT_COMMAND
	_tenant_pre() { return; }
	# shellcheck disable=SC2317
	_tenant_post() { return; }
	unset -f _tenant_classify _tenant_caps _tenant_glyph _tenant_time _tenant_arm \
		_tenant_disarm _tenant_ghost _tenant_up_fn _tenant_probe_up _tenant_cnf \
		_tenant_cnf_orig _tenant_unload
	unset _tenant_bin _tenant_dir _tenant_st _tenant_hist _tenant_cmd _tenant_shape \
		_tenant_first _tenant_live _tenant_tick _tenant_ps1 _tenant_ps1_mod _tenant_pwd \
		_tenant_pwdb _tenant_rtime _tenant_up_csi _tenant_up_ss3 _tenant_armed _tenant_capv \
		_tenant_r _tenant_loaded TENANT_HOOK
	return 0
}

if [[ -z $_tenant_armed ]]; then
	_tenant_probe_up
fi

# An exported PROMPT_COMMAND would stop being exported if it became an
# array, and child processes would lose it. In that case tenant stays out
# of this shell, and `tenant doctor` explains why.
if [[ ${PROMPT_COMMAND+${PROMPT_COMMAND@a}} == *x* ]]; then
	export TENANT_HOOK=bash-exported-prompt-command
else
	_tenant_keep=()
	for _tenant_e in ${PROMPT_COMMAND[@]+"${PROMPT_COMMAND[@]}"}; do
		[[ $_tenant_e == _tenant_pre || $_tenant_e == _tenant_post ]] || _tenant_keep+=("$_tenant_e")
	done
	PROMPT_COMMAND=(_tenant_pre ${_tenant_keep[@]+"${_tenant_keep[@]}"} _tenant_post)
	unset _tenant_keep _tenant_e
	export TENANT_HOOK=bash
fi

fi
