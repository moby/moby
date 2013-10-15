%{
// go tool yacc -o buildfile_parser.go buildfile.y 

package docker

import (
	"strings"
	"strconv"
)
%}

%union {
	i int
	ia []int
	s string
	sa []string
	v interface{}
	va []interface{}
}

%token <s> tokId
%token <i> tokInt
%token <s> tokStr
%token <s> tokArg

%type <va> instructions
%type <sa> shellArgs condArgs execArgs
%type <sa> spaceArgList commaArgList
%type <ia> ints

%type <v> instruction 
%type <v> fromInstr
%type <v> maintainerInstr
%type <v> runInstr
%type <v> commandInstr
%type <v> exposeInstr
%type <v> envInstr
%type <v> addInstr
%type <v> entryPointInstr
%type <v> volumeInstr
%type <v> userInstr
%type <v> workDirInstr
%type <s> arg

%token tokComma
%token tokColon
%token tokLBracket
%token tokRBracket
%token tokFrom
%token tokMaintainer
%token tokCommand
%token tokRun
%token tokExpose
%token tokEnv
%token tokAdd
%token tokEntryPoint
%token tokVolume
%token tokUser
%token tokWorkDir

%%

buildFile : 
	instructions 
	{
		parsedFile = &dockerFile{$1}
	}

instructions : 
	instructions instruction 
	{
		if $2 == nil {
			$$ = make([]interface{}, 0)
			$$ = append($$, $1)
		} else {
			$$ = $1
			$$ = append($1, $2)
		}
	} 
|	instruction 
	{
		if $1 == nil {
			$$ = []interface{}{}
		} else {
			$$ = make([]interface{}, 0)
			$$ = append($$, $1)
		}
	}

instruction: 
	fromInstr 
|	maintainerInstr
|	commandInstr
|	runInstr
|	exposeInstr
|	envInstr
|	addInstr
|	entryPointInstr
|	volumeInstr
|	userInstr
|	workDirInstr

fromInstr: 
	tokFrom arg
	{
		$$ = &fromArgs{
			name: $2,
		}
	}
|	tokFrom arg tokColon arg
	{
		$$ = &fromArgs{
			name: $2 + ":" + $4,
		}		
	}

maintainerInstr: 
	tokMaintainer spaceArgList
	{
		$$ = &maintainerArgs{
			name: strings.Join($2, " "),
		}
	}

runInstr: 
	tokRun shellArgs
	{
		$$ = &runArgs{
			cmd: fixcmd($2),
		}
	}
|	tokRun execArgs
	{
		$$ = &runArgs{
			cmd: $2,
		}
	}
|	tokRun condArgs
	{
		$$ = &runArgs{
			cmd: fixcmd($2),
		}
	}

commandInstr: 
	tokCommand shellArgs
	{
		$$ = &cmdArgs{
			cmd: fixcmd($2),
		}
	}
|	tokCommand execArgs
	{
		$$ = &cmdArgs{
			cmd: $2,
		}
	}
|	tokCommand condArgs
	{
		$$ = &cmdArgs{
			cmd: fixcmd($2),
		}
	}

exposeInstr:
	tokExpose ints 
	{
		$$ = &exposeArgs{
			ports: $2,
		}
	}

envInstr:
	tokEnv tokId arg
	{
		$$ = &envArgs{
			key:   $2,
			value: $3,
		}
	}

addInstr:
	tokAdd arg arg 
	{
		$$ = &addArgs{
			src: $2,
			dst: $3,
		}	
	}

entryPointInstr: 
	tokEntryPoint shellArgs
	{
		$$ = &entryPointArgs{
			cmd: fixcmd($2),
		}
	}
|	tokEntryPoint execArgs
	{
		$$ = &entryPointArgs{
			cmd: $2,
		}
	}
|	tokEntryPoint condArgs
	{
		$$ = &entryPointArgs{
			cmd: $2,
		}
	}

volumeInstr:
	tokVolume shellArgs
	{
		$$ = &volumeArgs{
			volumes: $2,
		}	
	}
|	tokVolume execArgs
	{
		$$ = &volumeArgs{
			volumes: $2,
		}	
	}

userInstr: 
	tokUser arg 
	{
		$$ = &userArgs{
			name: $2,
		}
	}

workDirInstr:
	tokWorkDir arg 
	{
		$$ = &workDirArgs{
			path: $2,
		}	
	}

/* 
We have 3 types of argument lists here:
1. [ foo bar baz ] -- shell condition eval
2. [ foo, bar, baz ] -- docker arguments to exec
3. foo bar baz -- docker arguments to /bin/sh -c
*/

condArgs: 
	tokLBracket spaceArgList tokRBracket 
	{
		$$ = append([]string{"["}, append($2, "]")...)
	}

shellArgs: 
	spaceArgList

execArgs: 
	tokLBracket commaArgList tokRBracket 
	{
		$$ = $2
	}

spaceArgList:
	spaceArgList arg 
	{
		$$ = append($1, $2)
	} 
|	arg 
	{
		if $1 == "" {
			$$ = []string{}
		} else {
			$$ = []string{$1}
		}
	}

commaArgList:
	commaArgList tokComma arg 
	{
		$$ = append($1, $3)
	} 
|	arg tokComma arg
	{
		$$ = []string{$1, $3}
	}

arg: 
	tokArg 
|	tokId
|	tokStr
|	tokInt 
	{
		$$ = strconv.Itoa($1)
	}

ints: 
	ints tokInt 
	{
		$$ = append($1, $2)
	} 
|	tokInt 
	{
		if $1 == 0 {
			$$ = []int{}
		} else {
			$$ = []int{$1}
		}
	}

%%

