# tenant: the zsh hook.
#
# This is everything tenant does inside your shell. It uses only zsh
# builtins. It never evals anything tenant prints: tenant answers with a verb
# and integers, and this script decides what to do with them. It does
# nothing at all until you run `tenant start`, and nothing as root or with
# TENANT_OFF set. `tenant evict` switches it off in every shell at the next
# prompt; then delete the eval line from ~/.zshrc.
#
# Every function sets its own options locally, so your setopts (ksharrays,
# globsubst, nounset, errexit, ...) can't change what it does. Sourcing this
# again keeps tenant's state and re-attaches the hooks.

if [[ -o interactive ]]; then

if [[ -z ${_tenant_loaded-} ]]; then
	typeset -g _tenant_loaded=1
	typeset -gi _tenant_st=0 _tenant_first=1 _tenant_live=0 _tenant_capv=0
	typeset -g _tenant_cmd='' _tenant_shape=- _tenant_armed='' _tenant_skip=''
	typeset -g _tenant_up_csi='' _tenant_up_ss3=''
	typeset -g _tenant_prompt='' _tenant_prompt_mod='' _tenant_rprompt='' _tenant_rprompt_mod=''
	typeset -g _tenant_pwd='' _tenant_pwda='' _tenant_pwdb='' _tenant_rtime='' _tenant_esc=''
fi
typeset -g _tenant_bin=__TENANT_BIN__
typeset -g _tenant_dir=${XDG_STATE_HOME:-$HOME/.local/state}/tenant
[[ $_tenant_dir == /* ]] || _tenant_dir=$HOME/.local/state/tenant
export TENANT_HOOK=zsh

# Runs first each prompt: keep $? and undo last prompt's change.
_tenant_status() {
	_tenant_st=$?
	setopt localoptions noksharrays noshwordsplit noglobsubst nowarncreateglobal unset noerrexit noerrreturn
	if [[ -n $_tenant_prompt_mod && $PROMPT == "$_tenant_prompt_mod" ]]; then
		PROMPT=$_tenant_prompt
	fi
	if [[ -n $_tenant_rprompt_mod && ${RPROMPT-} == "$_tenant_rprompt_mod" ]]; then
		RPROMPT=$_tenant_rprompt
	fi
	_tenant_prompt_mod='' _tenant_rprompt_mod=''
	return $_tenant_st
}

# Learn the first word of the command about to run. Nothing else about it
# leaves the shell. Commands starting with a space are skipped when
# HIST_IGNORE_SPACE is set, typos included.
_tenant_preexec() {
	setopt localoptions noksharrays noshwordsplit noglobsubst nowarncreateglobal unset noerrexit noerrreturn
	_tenant_cmd='' _tenant_shape=- _tenant_skip=''
	[[ -e $_tenant_dir/active ]] || return 0
	if [[ -o hist_ignore_space && $1 == ' '* ]]; then
		_tenant_skip=1
		return 0
	fi
	_tenant_classify "$1"
	return 0
}

_tenant_classify() {
	emulate -L zsh
	setopt noerrexit noerrreturn
	local -a words
	words=(${(z)1})
	local w=${words[1]-} x a long='' color='' all='' bad=''
	w=${w#\\}
	x=$w
	if [[ -n $w ]] && (( ${+aliases[$w]} )); then
		words=(${(z)aliases[$w]} ${words[2,-1]})
		x=${words[1]-}
	fi
	w=${w:t} x=${x:t}
	[[ $w =~ '^[A-Za-z0-9._+:@-]{1,40}$' ]] || return 0
	_tenant_cmd=$w
	case $x in
	(ls|eza|exa)
		[[ $x != ls ]] && color=c
		for a in ${words[2,-1]}; do
			case $a in
			(--color=never|--colour=never) color='' ;;
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
	setopt noerrexit noerrreturn
	local c=0 loc=${LC_ALL:-${LC_CTYPE:-${LANG-}}}
	[[ $PROMPT == *(%~|%/|%d|%1~|%c|%C|%.)* ]] && (( c |= 1 ))
	[[ -n $_tenant_up_csi$_tenant_up_ss3 ]] && (( c |= 2 ))
	[[ $loc == *[Uu][Tt][Ff](-|)8* ]] && (( c |= 4 ))
	case ${TERM-} in
	(xterm*|rxvt*|alacritty*|foot*|kitty*|wezterm*|st-*|konsole*|gnome*|vte*|contour*|ghostty*) (( c |= 8 )) ;;
	esac
	[[ -z ${RPROMPT-} ]] && (( c |= 16 ))
	[[ ${functions[command_not_found_handler]-} == *_tenant_cnf* ]] && (( c |= 32 ))
	_tenant_capv=$c
	return 0
}

# Runs last each prompt, after prompt frameworks have built PROMPT.
_tenant_precmd() {
	setopt localoptions noksharrays noshwordsplit noglobsubst nowarncreateglobal unset noerrexit noerrreturn
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
	local out verb a b c rc line
	local -a f
	out=$("$_tenant_bin" _hook prompt --shell zsh --pid "$$" --status "$st" --cmd "$_tenant_cmd" \
		--shape "$_tenant_shape" --first "$_tenant_first" --cols "${COLUMNS:-80}" --caps "$_tenant_capv")
	rc=$?
	if (( rc == 126 || rc == 127 )); then
		_tenant_unload # the binary has gone: stop quietly
		return $st
	fi
	_tenant_first=0
	_tenant_cmd='' _tenant_shape=-
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

# Escape text for use in a prompt, into _tenant_esc: % always, and ! when
# PROMPT_BANG is set. (With PROMPT_SUBST, prompts get a variable reference
# instead, which zsh doesn't expand again.)
_tenant_escape() {
	setopt localoptions noksharrays noshwordsplit noglobsubst nowarncreateglobal unset noerrexit noerrreturn
	local v=${1//\%/%%}
	[[ -o prompt_bang ]] && v=${v//\!/!!}
	_tenant_esc=$v
	return 0
}

# prompt.glyph: show the directory with one letter changed, for one prompt.
# Arguments are integers only: position from the end, the letter there, and
# the letter to show instead.
_tenant_glyph() {
	setopt localoptions noksharrays noshwordsplit noglobsubst nowarncreateglobal unset noerrexit noerrreturn
	[[ $1 == <-> && $2 == <-> && $3 == <-> ]] || return 0
	(( $2 >= 97 && $2 <= 122 && $3 >= 97 && $3 <= 122 )) || return 0
	local p=${(D)PWD} abs=$PWD f=${(#)2} t=${(#)3} i j
	i=$(( ${#p} - $1 + 1 ))
	j=$(( ${#abs} - $1 + 1 ))
	(( i > 1 && j > 1 )) || return 0
	[[ ${p[i]} == "$f" && ${abs[j]} == "$f" ]] || return 0
	p[i]=$t
	abs[j]=$t
	local full base absolute
	_tenant_escape "$p"; full=$_tenant_esc
	_tenant_escape "${p:t}"; base=$_tenant_esc
	_tenant_escape "$abs"; absolute=$_tenant_esc
	_tenant_prompt=$PROMPT
	if [[ -o prompt_subst ]]; then
		_tenant_pwd=$full _tenant_pwdb=$base _tenant_pwda=$absolute
		full='${_tenant_pwd}' base='${_tenant_pwdb}' absolute='${_tenant_pwda}'
	fi
	PROMPT=${PROMPT//'%~'/$full}
	PROMPT=${PROMPT//\%\//$absolute}
	PROMPT=${PROMPT//'%d'/$absolute}
	PROMPT=${PROMPT//'%1~'/$base}
	PROMPT=${PROMPT//'%c'/$base}
	PROMPT=${PROMPT//'%C'/$base}
	PROMPT=${PROMPT//'%.'/$base}
	_tenant_prompt_mod=$PROMPT
	return 0
}

# prompt.time: a time in RPROMPT, for one prompt.
_tenant_time() {
	setopt localoptions noksharrays noshwordsplit noglobsubst nowarncreateglobal unset noerrexit noerrreturn
	local t
	t=$("$_tenant_bin" _text time --pid "$$") || return 0
	[[ -n $t && ${#t} -le 20 && $t != *[[:cntrl:]]* ]] || return 0
	_tenant_escape "$t"
	_tenant_rprompt=${RPROMPT-}
	if [[ -o prompt_subst ]]; then
		_tenant_rtime=$_tenant_esc
		RPROMPT='${_tenant_rtime}'
	else
		RPROMPT=$_tenant_esc
	fi
	_tenant_rprompt_mod=$RPROMPT
	return 0
}

# history.ghost: borrow Up for one press. The line is filled in the editor
# only; it never goes into the history list or the history file. Each Up key
# is only borrowed if tenant knows what to put back.
_tenant_ghost() {
	setopt localoptions noksharrays noshwordsplit noglobsubst nowarncreateglobal unset noerrexit noerrreturn
	local g
	g=$("$_tenant_bin" _ghost --pid "$$" 2>/dev/null)
	_tenant_disarm
	if [[ -n $g && $g != *[[:cntrl:]]* ]]; then
		BUFFER=$g
		CURSOR=${#BUFFER}
	else
		zle ${_tenant_up_csi:-${_tenant_up_ss3:-up-line-or-history}}
	fi
	return 0
}
zle -N _tenant_ghost

_tenant_arm() {
	setopt localoptions noksharrays noshwordsplit noglobsubst nowarncreateglobal unset noerrexit noerrreturn
	[[ -n $_tenant_up_csi ]] && bindkey '^[[A' _tenant_ghost
	[[ -n $_tenant_up_ss3 ]] && bindkey '^[OA' _tenant_ghost
	[[ -n $_tenant_up_csi$_tenant_up_ss3 ]] && _tenant_armed=1
	return 0
}

_tenant_disarm() {
	setopt localoptions noksharrays noshwordsplit noglobsubst nowarncreateglobal unset noerrexit noerrreturn
	[[ -n $_tenant_up_csi ]] && bindkey '^[[A' $_tenant_up_csi
	[[ -n $_tenant_up_ss3 ]] && bindkey '^[OA' $_tenant_up_ss3
	_tenant_armed=''
	return 0
}

# Which widget each Up key runs, so it can be put back. Only well-known
# history widgets: keys owned by other tools (atuin, mcfly, fzf) are left
# alone.
_tenant_probe_up() {
	emulate -L zsh
	setopt noerrexit noerrreturn
	local key w
	for key in '^[[A' '^[OA'; do
		w=$(bindkey "$key" 2>/dev/null)
		w=${w##* }
		case $w in
		(up-line-or-history|up-history|up-line-or-beginning-search|up-line-or-search|history-substring-search-up|history-beginning-search-backward) ;;
		(*) w='' ;;
		esac
		if [[ $key == '^[[A' ]]; then _tenant_up_csi=$w; else _tenant_up_ss3=$w; fi
	done
	return 0
}

# notfound.remark: wrap any existing handler, let it run first, unchanged,
# then let tenant add a line.
if (( ${+functions[command_not_found_handler]} )) && [[ ${functions[command_not_found_handler]} != *_tenant_cnf* ]]; then
	functions[_tenant_cnf_orig]=$functions[command_not_found_handler]
fi

_tenant_cnf() {
	setopt localoptions noksharrays noshwordsplit noglobsubst nowarncreateglobal unset noerrexit noerrreturn
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
			"$_tenant_bin" _hook notfound --shell zsh --pid "$$" --cmd "$w" --caps "$_tenant_capv" >/dev/null
	fi
	return $rc
}

command_not_found_handler() { _tenant_cnf "$@" }

# Put everything back the way it was. Only undoes what is still tenant's:
# a not-found handler defined later is left alone.
_tenant_unload() {
	setopt localoptions noksharrays noshwordsplit noglobsubst nowarncreateglobal unset noerrexit noerrreturn
	[[ -n $_tenant_armed ]] && _tenant_disarm
	if [[ -n $_tenant_prompt_mod && $PROMPT == "$_tenant_prompt_mod" ]]; then
		PROMPT=$_tenant_prompt
	fi
	if [[ -n $_tenant_rprompt_mod && ${RPROMPT-} == "$_tenant_rprompt_mod" ]]; then
		RPROMPT=$_tenant_rprompt
	fi
	if [[ ${functions[command_not_found_handler]-} == *_tenant_cnf* ]]; then
		if (( ${+functions[_tenant_cnf_orig]} )); then
			functions[command_not_found_handler]=$functions[_tenant_cnf_orig]
		else
			unfunction command_not_found_handler 2>/dev/null
		fi
	fi
	precmd_functions=(${precmd_functions:#(_tenant_status|_tenant_precmd)})
	preexec_functions=(${preexec_functions:#_tenant_preexec})
	zle -D _tenant_ghost 2>/dev/null
	unfunction _tenant_status _tenant_preexec _tenant_classify _tenant_caps _tenant_precmd \
		_tenant_escape _tenant_glyph _tenant_time _tenant_ghost _tenant_arm _tenant_disarm \
		_tenant_probe_up _tenant_cnf _tenant_unload 2>/dev/null
	(( ${+functions[_tenant_cnf_orig]} )) && unfunction _tenant_cnf_orig
	unset _tenant_bin _tenant_dir _tenant_st _tenant_first _tenant_live _tenant_capv \
		_tenant_cmd _tenant_shape _tenant_armed _tenant_up_csi _tenant_up_ss3 _tenant_prompt \
		_tenant_prompt_mod _tenant_rprompt _tenant_rprompt_mod _tenant_pwd _tenant_pwda \
		_tenant_pwdb _tenant_rtime _tenant_esc _tenant_skip _tenant_loaded TENANT_HOOK
	return 0
}

# Attach (again): first and last in precmd, last in preexec.
() {
	setopt localoptions noksharrays noshwordsplit noglobsubst nowarncreateglobal unset
	[[ -n $_tenant_armed ]] || _tenant_probe_up
	precmd_functions=(_tenant_status ${precmd_functions:#(_tenant_status|_tenant_precmd)} _tenant_precmd)
	preexec_functions=(${preexec_functions:#_tenant_preexec} _tenant_preexec)
}

fi
