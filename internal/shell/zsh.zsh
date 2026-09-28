# tenant: the zsh hook.
#
# This is everything tenant does inside your shell. It uses only zsh
# builtins. It never evals anything tenant prints: tenant answers with a verb
# and integers, and this script decides what to do with them. It does
# nothing at all until you run `tenant start`, and nothing as root or with
# TENANT_OFF set. `tenant evict` switches it off in every shell at the next
# prompt; then delete the eval line from ~/.zshrc.

if [[ -o interactive && -z ${_tenant_loaded-} ]]; then

typeset -g _tenant_loaded=1
typeset -g _tenant_bin=__TENANT_BIN__
typeset -g _tenant_dir=${XDG_STATE_HOME:-$HOME/.local/state}/tenant
[[ $_tenant_dir == /* ]] || _tenant_dir=$HOME/.local/state/tenant
typeset -gi _tenant_st=0 _tenant_first=1 _tenant_live=0 _tenant_capv=0
typeset -g _tenant_cmd= _tenant_shape=- _tenant_armed= _tenant_up= _tenant_skip=
typeset -g _tenant_prompt= _tenant_prompt_mod= _tenant_rprompt= _tenant_rprompt_mod=
typeset -g _tenant_pwd= _tenant_pwdb= _tenant_rtime=
export TENANT_HOOK=zsh

# Runs first each prompt: keep $? and undo last prompt's change.
_tenant_status() {
	_tenant_st=$?
	if [[ -n $_tenant_prompt_mod && $PROMPT == $_tenant_prompt_mod ]]; then
		PROMPT=$_tenant_prompt
	fi
	if [[ -n $_tenant_rprompt_mod && $RPROMPT == $_tenant_rprompt_mod ]]; then
		RPROMPT=$_tenant_rprompt
	fi
	_tenant_prompt_mod= _tenant_rprompt_mod=
	return $_tenant_st
}

# Learn the first word of the command about to run. Nothing else about it
# leaves the shell. Commands starting with a space are skipped when
# HIST_IGNORE_SPACE is set.
_tenant_preexec() {
	_tenant_cmd= _tenant_shape=- _tenant_skip=
	[[ -e $_tenant_dir/active ]] || return 0
	if [[ -o hist_ignore_space && $1 == ' '* ]]; then
		_tenant_skip=1
		return 0
	fi
	_tenant_classify "$1"
}

_tenant_classify() {
	emulate -L zsh
	local -a words
	words=(${(z)1})
	local w=${words[1]#\\} x a long= color= all= bad=
	x=$w
	if (( ${+aliases[$w]} )); then
		words=(${(z)aliases[$w]} ${words[2,-1]})
		x=${words[1]}
	fi
	w=${w:t} x=${x:t}
	[[ $w =~ '^[A-Za-z0-9._+:@-]{1,40}$' ]] || return 0
	_tenant_cmd=$w
	case $x in
	(ls|eza|exa)
		[[ $x != ls ]] && color=c
		for a in ${words[2,-1]}; do
			case $a in
			(--color=never|--colour=never) color= ;;
			(--color*|--colour*) color=c ;;
			(--icons*) bad=x ;;
			(--long|--format=long|--format=verbose) long=l ;;
			(--all|--almost-all) all=a ;;
			(--*) ;;
			(-*)
				[[ $a == *[^-A-Za-z0-9=,]* ]] && bad=x
				[[ $a == *l* ]] && long=l
				[[ $a == *[aA]* ]] && all=a
				[[ $a == *[Rd]* ]] && bad=x
				;;
			(*) bad=x ;;
			esac
		done
		if [[ -n $bad ]]; then _tenant_shape=ls:x; else _tenant_shape=ls:${long:-p}$color$all; fi
		;;
	(clear) _tenant_shape=clear ;;
	(cd) _tenant_shape=cd ;;
	esac
	return 0
}

# What this shell can show, as a bitmask (see internal/mech).
_tenant_caps() {
	emulate -L zsh
	local c=0 loc=${LC_ALL:-${LC_CTYPE:-${LANG-}}}
	[[ $PROMPT == *(%~|%/|%d|%1~|%c|%C|%.)* ]] && (( c |= 1 ))
	[[ -n $_tenant_up ]] && (( c |= 2 ))
	[[ $loc == *[Uu][Tt][Ff](-|)8* ]] && (( c |= 4 ))
	case ${TERM-} in
	(xterm*|rxvt*|alacritty*|foot*|kitty*|wezterm*|st-*|konsole*|gnome*|vte*|contour*|ghostty*) (( c |= 8 )) ;;
	esac
	[[ -z $RPROMPT ]] && (( c |= 16 ))
	(( ${+functions[_tenant_cnf]} )) && (( c |= 32 ))
	_tenant_capv=$c
}

# Runs last each prompt, after prompt frameworks have built PROMPT.
_tenant_precmd() {
	local st=$_tenant_st
	[[ -n $_tenant_armed ]] && _tenant_disarm
	if [[ ! -e $_tenant_dir/active ]]; then
		if (( _tenant_live )); then
			_tenant_unload
			return $st
		fi
		_tenant_first=0
		return $st
	fi
	if [[ -n ${TENANT_OFF-} ]] || (( EUID == 0 )); then
		_tenant_first=0
		return $st
	fi
	_tenant_live=1
	_tenant_caps
	local out verb a b c rc
	out=$("$_tenant_bin" _hook prompt --shell zsh --status "$st" --cmd "$_tenant_cmd" \
		--shape "$_tenant_shape" --first "$_tenant_first" --cols "${COLUMNS:-80}" --caps "$_tenant_capv")
	rc=$?
	if (( rc == 126 || rc == 127 )); then
		_tenant_unload # the binary has gone: stop quietly
		return $st
	fi
	_tenant_first=0
	_tenant_cmd= _tenant_shape=-
	local line
	local -a f
	for line in ${(f)out}; do
		f=(${=line})
		verb=${f[1]-} a=${f[2]-} b=${f[3]-} c=${f[4]-}
		case $verb in
		(glyph) _tenant_glyph "$a" "$b" "$c" ;;
		(time) _tenant_time ;;
		(ghost) _tenant_arm ;;
		esac
	done
	return $st
}

# Escape text for use in a prompt: % always, and $ ` \ when PROMPT_SUBST
# would otherwise expand them, and ! when PROMPT_BANG is set.
_tenant_escape() {
	local v=${1//\%/%%}
	[[ -o prompt_bang ]] && v=${v//\!/!!}
	REPLY=$v
}

# prompt.glyph: show the directory with one letter changed, for one prompt.
# Arguments are integers only.
_tenant_glyph() {
	[[ $1 == <-> && $2 == <-> && $3 == <-> ]] || return
	(( $2 >= 97 && $2 <= 122 && $3 >= 97 && $3 <= 122 )) || return
	local p=${(D)PWD} i f=${(#)2} t=${(#)3}
	i=$(( ${#p} - $1 + 1 ))
	(( i > 1 )) || return
	[[ ${p[i]} == $f ]] || return
	p[i]=$t
	_tenant_escape "$p"
	local full=$REPLY
	_tenant_escape "${p:t}"
	local base=$REPLY
	_tenant_prompt=$PROMPT
	if [[ -o prompt_subst ]]; then
		_tenant_pwd=$full _tenant_pwdb=$base
		full='${_tenant_pwd}' base='${_tenant_pwdb}'
	fi
	PROMPT=${PROMPT//'%~'/$full}
	PROMPT=${PROMPT//'%/'/$full}
	PROMPT=${PROMPT//'%d'/$full}
	PROMPT=${PROMPT//'%1~'/$base}
	PROMPT=${PROMPT//'%c'/$base}
	PROMPT=${PROMPT//'%C'/$base}
	PROMPT=${PROMPT//'%.'/$base}
	_tenant_prompt_mod=$PROMPT
}

# prompt.time: a time in RPROMPT, for one prompt.
_tenant_time() {
	local t
	t=$("$_tenant_bin" _text time) || return
	[[ -n $t && ${#t} -le 20 && $t != *[[:cntrl:]]* ]] || return
	_tenant_escape "$t"
	_tenant_rprompt=$RPROMPT
	if [[ -o prompt_subst ]]; then
		_tenant_rtime=$REPLY
		RPROMPT='${_tenant_rtime}'
	else
		RPROMPT=$REPLY
	fi
	_tenant_rprompt_mod=$RPROMPT
}

# history.ghost: borrow Up for one press. The line is filled in the editor
# only; it never goes into the history list or the history file.
_tenant_ghost() {
	local g
	g=$("$_tenant_bin" _ghost 2>/dev/null)
	_tenant_disarm
	if [[ -n $g && $g != *[[:cntrl:]]* ]]; then
		BUFFER=$g
		CURSOR=${#BUFFER}
	else
		zle $_tenant_up
	fi
}
zle -N _tenant_ghost

_tenant_arm() {
	[[ -n $_tenant_up ]] || return
	bindkey '^[[A' _tenant_ghost
	bindkey '^[OA' _tenant_ghost
	_tenant_armed=1
}

_tenant_disarm() {
	bindkey '^[[A' $_tenant_up
	bindkey '^[OA' $_tenant_up
	_tenant_armed=
}

# Which widget Up runs, so it can be put back. Up keys owned by other tools
# (atuin, mcfly, fzf) are left alone.
_tenant_probe_up() {
	local b w
	b=$(bindkey '^[[A' 2>/dev/null)
	w=${b##* }
	case $w in
	(up-line-or-history|up-history|up-line-or-beginning-search|up-line-or-search|history-substring-search-up|history-beginning-search-backward)
		_tenant_up=$w ;;
	esac
}

# notfound.remark: wrap any existing handler, let it run first, unchanged,
# then let tenant add a line.
if (( ${+functions[command_not_found_handler]} )); then
	functions[_tenant_cnf_orig]=$functions[command_not_found_handler]
fi

_tenant_cnf() {
	local rc=127
	if (( ${+functions[_tenant_cnf_orig]} )); then
		_tenant_cnf_orig "$@"
		rc=$?
	else
		print -r -u2 -- "zsh: command not found: $1"
	fi
	if [[ -e $_tenant_dir/active && -z ${TENANT_OFF-} && -z $_tenant_skip ]] && (( EUID != 0 )); then
		local w=${1:t}
		[[ $w =~ '^[A-Za-z0-9._+:@-]{1,40}$' ]] &&
			"$_tenant_bin" _hook notfound --shell zsh --cmd "$w" --caps "$_tenant_capv" >/dev/null
	fi
	return $rc
}

command_not_found_handler() { _tenant_cnf "$@" }

# Put everything back the way it was.
_tenant_unload() {
	[[ -n $_tenant_armed ]] && _tenant_disarm
	if [[ -n $_tenant_prompt_mod && $PROMPT == $_tenant_prompt_mod ]]; then
		PROMPT=$_tenant_prompt
	fi
	if [[ -n $_tenant_rprompt_mod && $RPROMPT == $_tenant_rprompt_mod ]]; then
		RPROMPT=$_tenant_rprompt
	fi
	if (( ${+functions[_tenant_cnf_orig]} )); then
		functions[command_not_found_handler]=$functions[_tenant_cnf_orig]
	else
		unfunction command_not_found_handler 2>/dev/null
	fi
	precmd_functions=(${precmd_functions:#(_tenant_status|_tenant_precmd)})
	preexec_functions=(${preexec_functions:#_tenant_preexec})
	zle -D _tenant_ghost 2>/dev/null
	unfunction _tenant_status _tenant_preexec _tenant_classify _tenant_caps _tenant_precmd \
		_tenant_escape _tenant_glyph _tenant_time _tenant_ghost _tenant_arm _tenant_disarm \
		_tenant_probe_up _tenant_cnf _tenant_unload 2>/dev/null
	(( ${+functions[_tenant_cnf_orig]} )) && unfunction _tenant_cnf_orig
	unset _tenant_bin _tenant_dir _tenant_st _tenant_first _tenant_live _tenant_capv \
		_tenant_cmd _tenant_shape _tenant_armed _tenant_up _tenant_prompt _tenant_prompt_mod \
		_tenant_rprompt _tenant_rprompt_mod _tenant_pwd _tenant_pwdb _tenant_rtime \
		_tenant_skip _tenant_loaded TENANT_HOOK
}

_tenant_probe_up
precmd_functions=(_tenant_status $precmd_functions _tenant_precmd)
preexec_functions=($preexec_functions _tenant_preexec)

fi
