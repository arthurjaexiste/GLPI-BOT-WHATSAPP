package main

import (
	"fmt"
	"reflect"
	"go.mau.fi/whatsmeow/types"
)

func main() {
	var info types.ProfilePictureInfo
	t := reflect.TypeOf(info)
	fmt.Println("Fields in types.ProfilePictureInfo:")
	for i := 0; i < t.NumField(); i++ {
		fmt.Printf("Field: %s, Type: %s\n", t.Field(i).Name, t.Field(i).Type)
	}
}
