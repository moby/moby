" dockerfile.vim - Syntax highlighting for Dockerfiles
" Maintainer:   Honza Pokorny <http://honza.ca>
" Version:      0.5


if exists("b:current_syntax")
    finish
endif

let b:current_syntax = "dockerfile"

syntax case ignore

syntax match dockerfileKeyword /\v^\s*(FROM|MAINTAINER|RUN|CMD|EXPOSE|ENV|ADD)\s/
syntax match dockerfileKeyword /\v^\s*(ENTRYPOINT|VOLUME|USER|WORKDIR)\s/
highlight link dockerfileKeyword Keyword

syntax region dockerfileString start=/\v"/ skip=/\v\\./ end=/\v"/
highlight link dockerfileString String

syntax match dockerfileComment "\v^\s*#.*$"
highlight link dockerfileComment Comment
