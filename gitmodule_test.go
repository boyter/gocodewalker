package gocodewalker

import (
	"reflect"
	"testing"
)

func Test_extractGitModuleFolders(t *testing.T) {
	type args struct {
		input string
	}
	tests := []struct {
		name string
		args args
		want []string
	}{
		{
			name: "",
			args: args{
				input: `[submodule "contracts/lib/forge-std"]
	path = contracts/lib/forge-std
	url = https://github.com/foundry-rs/forge-std
	tag = v1.8.2
[submodule "contracts/lib/sp1-contracts"]
	path = contracts/lib/sp1-contracts
	url = https://github.com/succinctlabs/sp1-contracts
	tag = main
[submodule "contracts/lib/openzeppelin-contracts"]
	path = contracts/lib/openzeppelin-contracts
	url = https://github.com/OpenZeppelin/openzeppelin-contracts
[submodule "lib/java-tron"]
	path = lib/java-tron
	url = https://github.com/tronprotocol/java-tron
[submodule "lib/googleapis"]
	path = lib/googleapis
	url = https://github.com/googleapis/googleapis
[submodule "contracts/lib/openzeppelin-contracts-upgradeable"]
	path = contracts/lib/openzeppelin-contracts-upgradeable
	url = https://github.com/OpenZeppelin/openzeppelin-contracts-upgradeable
[submodule "contracts/lib/v2-testnet-contracts"]
	path = contracts/lib/v2-testnet-contracts
	url = https://github.com/matter-labs/v2-testnet-contracts
	branch = beta`,
			},
			want: []string{
				"contracts/lib/forge-std",
				"contracts/lib/sp1-contracts",
				"contracts/lib/openzeppelin-contracts",
				"lib/java-tron",
				"lib/googleapis",
				"contracts/lib/openzeppelin-contracts-upgradeable",
				"contracts/lib/v2-testnet-contracts",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := extractGitModuleFolders(tt.args.input); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("extractGitModuleFolders() = %v, want %v", got, tt.want)
			}
		})
	}
}
