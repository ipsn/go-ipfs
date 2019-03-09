package commands

import (
	cmdenv "github.com/ipsn/go-ipfs/core/commands/cmdenv"

	"github.com/ipsn/go-ipfs/gxlibs/github.com/ipfs/go-ipfs-cmdkit"
	cmds "github.com/ipsn/go-ipfs/gxlibs/github.com/ipfs/go-ipfs-cmds"
)

var daemonShutdownCmd = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline: "Shut down the ipfs daemon",
	},
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) error {
		nd, err := cmdenv.GetNode(env)
		if err != nil {
			return err
		}

		if !nd.IsDaemon {
			return cmdkit.Errorf(cmdkit.ErrClient, "daemon not running")
		}

		if err := nd.Process().Close(); err != nil {
			log.Error("error while shutting down ipfs daemon:", err)
		}

		return nil
	},
}
