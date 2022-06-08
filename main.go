package main

import (
	"fmt"
	"os/exec"
	//"terraform-provider-hashicups-pf/hashicups"
)

func main() {
	/*tfsdk.Serve(context.Background(), hashicups.New, tfsdk.ServeOpts{
		Name: "hashicups",
	})*/
	execute()
}

func execute() {

	// here we perform the pwd command.
	// we can store the output of this in our out variable
	// and catch any errors in err

	comande := "powershell.exe ./script.ps1 -Backendpool default-citeo-plus-be-pool"
	out, err := exec.Command(comande).Output()
	
	//comande := ".\\script.ps1 -Backendpool default-citeo-plus-be-pool"
	//out, err := exec.Command("powershell", "-NoProfile", comande).CombinedOutput()
	if err != nil {
        fmt.Printf("%s", err)
    }
	// if there is an error with our execution
	// handle it here
	/*if err != nil {

		log.Fatalf("cmd.Run() failed with %s\n", err)
	}*/
	fmt.Printf("out %s", out)
	// as the out variable defined above is of type []byte we need to convert
	// this to a string or else we will see garbage printed out in our console
	// this is how we convert it to a string

}