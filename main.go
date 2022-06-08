package main

import (
	//"context"
	"fmt"
	"log"
	"os/exec"
	"reflect"
	//"runtime"
	//"terraform-provider-hashicups-pf/hashicups"
	//"github.com/hashicorp/terraform-plugin-framework/tfsdk"
)

func main() {
	/*
	tfsdk.Serve(context.Background(), hashicups.New, tfsdk.ServeOpts{
		Name: "hashicups",
	})*/
	/*
	if runtime.GOOS == "windows" {
        fmt.Println("Can't Execute this on a windows machine")
    } else {
        execute()
    }*/
	execute()
}


func execute() {

    // here we perform the pwd command.
    // we can store the output of this in our out variable
    // and catch any errors in err
	
	
    comande := ".\\script.ps1 -Backendpool default-citeo-plus-be-pool"
	//out, err := exec.Command(comande).Output()
	out , err := exec.Command("powershell", "-NoProfile",comande).CombinedOutput()
	
    // if there is an error with our execution
    // handle it here
    if err != nil {

        log.Fatalf("cmd.Run() failed with %s\n", err)
    }
	fmt.Println(reflect.TypeOf(out))
	fmt.Printf("out %s",out)
    // as the out variable defined above is of type []byte we need to convert
    // this to a string or else we will see garbage printed out in our console
    // this is how we convert it to a string
    

}