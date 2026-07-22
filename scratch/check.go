// ============================================================================
// ARQUIVO: check.go
// Descrição: Implementação Go (backend) para o ecossistema GLPI-BOT.
// ============================================================================

package main

import (
	"fmt"
	"reflect"
	"go.mau.fi/whatsmeow/types"
)


// Função main executa a regra de negócio/rotina correspondente
func main() {
	var info types.ProfilePictureInfo
	t := reflect.TypeOf(info)
	fmt.Println("Fields in types.ProfilePictureInfo:")
	for i := 0; i < t.NumField(); i++ {
		fmt.Printf("Field: %s, Type: %s\n", t.Field(i).Name, t.Field(i).Type)
	}
}
