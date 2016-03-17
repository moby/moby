//This is NOT complete
//<pirabarlen cheenaramen> selven@hackers.mu
package ipset
import (

	"fmt"
	"errors"
	"os/exec"
	"strings"
	"bytes"
)
// ipset commands
type option string
type timeout uint
//

var (
	ipsetPath string

	//Possible Typename Methods
	//true if enabled, false otherwise
	TypeNameMethods = map[string]bool{
		"bitmap": true,
		"hash"  : true,
		"list"  : true,
	}

	//Possible values for Typename datatypes
	TypeNameDataType = map[string]bool{
		"ip"   : true,
		"net"  : true,
		"mac"  : true,
		"port" : true,
		"iface": true,
	}
	//Possible create options
	CREATE_OPTIONS_BITMAP = map [string]bool{
		"timeout" : true, //int
		//add netmask on bitmap:ip structure definition
		"range"   : true, //fromip-toip|ip/cidr , mandatory
		"counters": true, //flag
		"comment" : true, //flag

	}

	CREATE_OPTIONS_HASH = map[string] bool{
		"timeout" : true, //int
		//add "netmask" : true on hash:ip//cidr
		"counters": true, //flag
		"comment" : true, //flag
		"familly" : true, // inet
		"hashsize": true, //int, power of 2, default 1024, kernel rounds up non power of two.
		"maxelem" : true, //int
	}

	CREATE_OPTIONS_LIST = map [string] bool {
		"size"     : true, //int
		"timeout"  : true, //int
		"counters" : true, //flag
		"comment"  : true, //flag
	}

	CREATE_OPTIONS_EXCEPTIONS_VALUES = map[string] bool {
		"netmask" : true, //cidr
	}
	CREATE_OPTIONS_EXCEPTIONS = map[string] map[string]bool {
		"bitmap:ip_netmask" : CREATE_OPTIONS_EXCEPTIONS_VALUES,
		"hash:ip_netmask"   : CREATE_OPTIONS_EXCEPTIONS_VALUES,
	}

	CREATE_OPTIONS = map[string] map[string]bool {
		"bitmap:ip"           : CREATE_OPTIONS_BITMAP,
		"bitmap:ip,mac"       : CREATE_OPTIONS_BITMAP,
		"bitmap:ip,port"      : CREATE_OPTIONS_BITMAP,
		"hash:ip"             : CREATE_OPTIONS_HASH,
		"hash:ip,net"         : CREATE_OPTIONS_HASH,
		"hash:net"         : CREATE_OPTIONS_HASH,
		"hash:net,net"        : CREATE_OPTIONS_HASH,
		"hash:ip,port"        : CREATE_OPTIONS_HASH,
		"hash:net,port"       : CREATE_OPTIONS_HASH,
		"hash:ip,port,ip"     : CREATE_OPTIONS_HASH,
		"hash:ip,port,net"    : CREATE_OPTIONS_HASH,
		"hash:net,port,net"   : CREATE_OPTIONS_HASH,
		"hash:net,port,iface" : CREATE_OPTIONS_HASH,
		"list:set"            : CREATE_OPTIONS_LIST,
	}
	//ERRORS
	ErrIpsetNotFound = errors.New("ipset Not found")
	//can only be bitmap, hash, list
	ErrInvalidTNMethod = errors.New("Invalid TYPENAME method")
	ErrInvalidTNDataType= errors.New("Typename datatype incorrect")
	ErrInvalidCreateOptions= errors.New("Invalid create options supplied, make sure it is valid for your TYPENAME")
)

func init(){
	err:=initCheck()
	if err != nil {
		fmt.Println("Error:", err)

	}
}
func initCheck() error {
	if ipsetPath == "" {
		path, err:= exec.LookPath("ipset")
		if err != nil {
			return ErrIpsetNotFound
		}

		ipsetPath=path
	}
	return nil
}

// VERIFIES TYPENAME is according to ipset's requirement
func validTypeName(tnMethod string, tnDataType []string) (err error) {
	//Do we support that TYPENAME METHOD
	// bitmap, hash, list
	if ! TypeNameMethods[tnMethod]{
		return ErrInvalidTNMethod
	}
	// we can't allow more data types than supported  (ip, net, mac, port and iface )
	if len(tnDataType) > len(TypeNameDataType) {
		return ErrInvalidTNDataType
	}


	for _, tnDT := range tnDataType  {

		if !(TypeNameDataType[tnDT]){
			return ErrInvalidTNDataType
		}
	}
	return nil
}

func trimEntireSlice(aSlice []string) ([]string) {

	for index, element := range aSlice {
		aSlice[index]=strings.Trim(element, " ")
	}
	return aSlice
}

func generateTypeName(tnMethod string, tnDataType []string) (string ) {
	tnDataType=trimEntireSlice(tnDataType)
	return strings.Trim(tnMethod, " ")+":"+strings.Join (tnDataType[:], ",")
}

func validCreateOptions(tnMethod string, tnDataType []string, createOptions map[string] string) (err error) {
	//validate createOptions
	//we need to check the value of each create-option's value
	//ignore this for now
	vco_typeName:= generateTypeName(tnMethod, tnDataType)

	for key, value := range createOptions {
		vco_exception:=vco_typeName+"_"+key
		if ! (CREATE_OPTIONS[vco_typeName][key] || CREATE_OPTIONS_EXCEPTIONS[vco_exception][key]) {
			fmt.Println("value:", value) //replace with value checker
			 return ErrInvalidCreateOptions
		}
	}
	//no errors, valid create options
	return nil
}

func generateCreateOption(createOptions map[string] string) ([]string) {
	var gco_createOptions []string
	var buf string
	for key, value := range createOptions {
		trimmedValue:=strings.Trim(value, " ")
		if trimmedValue==""  {
			buf=strings.Trim(key, " ")
			gco_createOptions=append(gco_createOptions, buf)
		} else {
			buf= strings.Trim(key, " ")
			gco_createOptions=append(gco_createOptions, strings.Trim(key, " ")+" "+trimmedValue)
		}

	}
	return gco_createOptions
}


func Create(setname string, tnMethod string, tnDataType []string, createOptions map[string] string) (output string, err error) {
	//Output of our ipset create
	var out bytes.Buffer
	var arguments []string
	//verify if TYPENAME is valid, see ipset manpage
	//work towards deprecating this check
	err = validTypeName(tnMethod, tnDataType)
	if err != nil {
		 return "", err
	}

	// verify if CREATE-OPTIONS is valid, see ipset manpage
	err = validCreateOptions(tnMethod, tnDataType, createOptions)
	if err != nil {
		 return "", err
	}
	//reset error, as we are reusing it to return run failures
	err=nil
	//modify generateTypeName and generateCreateOptions to return a slice of string
	// use slice append(s1,s2) to merge to slices.
	//exec takes args as a slice also
	typeName:=generateTypeName(tnMethod, tnDataType)
	generatedCreateOptions:=generateCreateOption(createOptions)
	arguments=[]string{setname, typeName}
	arguments=append(arguments, generatedCreateOptions...)
	fmt.Println(arguments)
	cmd:=exec.Command(ipsetPath, arguments...)
	cmd.Stdout = &out
	err = cmd.Run()

	return out.String(), err


}

//should be called only by Save(), List(), Destroy()
func dlsAction(options []string, setAction string,setname string) (string, error) {
	var cmd *exec.Cmd
	var out bytes.Buffer
	if len(options)==0 {
		cmd=exec.Command(ipsetPath,setAction , setname)
	} else {
		return "",nil //TODO: just merge options and setname in one slice and pass to cmd
	}

	cmd.Stdout = &out
	err := cmd.Run()
	return out.String(), err
}

//supports only defaults for now
func Save(setname string, options []string) (string, error) {
	if len(options)==0 {
		return dlsAction(options,"save",setname)
	}
	return "",nil
}

func List(setname string, options []string) (string, error) {

	if len(options)==0 {
	return dlsAction(options, "list",setname)
	}
	return "",nil
}

func Destroy(setname string, options []string) (string, error) {

	if len(options)==0 {
	return dlsAction(options, "destroy",setname)
	}
	return "",nil
}

func Flush(setname string, options []string) (string, error) {

	if len(options)==0 {
	return dlsAction(options, "flush",setname)
	}
	return "",nil
}

