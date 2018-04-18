package commands

import (
	cmds "github.com/ipsn/go-ipfs/commands"

	"github.com/ipsn/go-ipfs/gxlibs/github.com/ipfs/go-ipfs-cmdkit"
)

var DiagCmd = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline: "Generate diagnostic reports.",
	},

	Subcommands: map[string]*cmds.Command{
		"sys":  sysDiagCmd,
		"cmds": ActiveReqsCmd,
	},
}
