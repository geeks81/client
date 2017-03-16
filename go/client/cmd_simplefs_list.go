// Copyright 2015 Keybase, Inc. All rights reserved. Use of
// this source code is governed by the included BSD license.

package client

import (
	"errors"

	"golang.org/x/net/context"

	"github.com/keybase/cli"
	"github.com/keybase/client/go/libcmdline"
	"github.com/keybase/client/go/libkb"
	keybase1 "github.com/keybase/client/go/protocol/keybase1"
)

// CmdSimpleFSList is the 'fs ls' command.
type CmdSimpleFSList struct {
	libkb.Contextified
	paths   []keybase1.Path
	recurse bool
}

// NewCmdSimpleFSList creates a new cli.Command.
func NewCmdSimpleFSList(cl *libcmdline.CommandLine, g *libkb.GlobalContext) cli.Command {
	return cli.Command{
		Name:         "ls",
		ArgumentHelp: "<path>",
		Usage:        "list directory contents",
		Action: func(c *cli.Context) {
			cl.ChooseCommand(&CmdSimpleFSList{Contextified: libkb.NewContextified(g)}, "ls", c)
		},
		Flags: []cli.Flag{
			cli.BoolFlag{
				Name:  "r, recursive",
				Usage: "recurse into subdirectories",
			},
		},
	}
}

// Run runs the command in client/server mode.
func (c *CmdSimpleFSList) Run() error {

	cli, err := GetSimpleFSClient(c.G())
	if err != nil {
		return err
	}

	ctx := context.TODO()

	paths, err := doSimpleFSGlob(c.G(), ctx, cli, c.paths)
	if err != nil {
		return err
	}

	// If the argument was globbed, we really just want a stat of each item
	if len(paths) > 1 {
		var listResult keybase1.SimpleFSListResult
		for _, path := range paths {
			e, err := cli.SimpleFSStat(context.TODO(), path)
			if err != nil {
				return err
			}
			// TODO: should stat include the path in the result?
			e.Name = pathToString(path)
			listResult.Entries = append(listResult.Entries, e)
		}
		c.output(listResult)
	} else if len(paths) == 1 {
		path := paths[0]
		c.G().Log.Debug("SimpleFSList %s", pathToString(path))

		opid, err := cli.SimpleFSMakeOpid(ctx)
		if err != nil {
			return err
		}
		defer cli.SimpleFSClose(ctx, opid)
		if c.recurse {
			err = cli.SimpleFSListRecursive(ctx, keybase1.SimpleFSListRecursiveArg{
				OpID: opid,
				Path: path,
			})
		} else {
			err = cli.SimpleFSList(ctx, keybase1.SimpleFSListArg{
				OpID: opid,
				Path: path,
			})
		}
		if err != nil {
			return err
		}

		err = cli.SimpleFSWait(ctx, opid)
		if err != nil {
			return err
		}
		for {
			listResult, err := cli.SimpleFSReadList(ctx, opid)
			if err != nil {
				break
			}
			c.output(listResult)
		}
	}
	return err
}

func (c *CmdSimpleFSList) output(listResult keybase1.SimpleFSListResult) {

	ui := c.G().UI.GetTerminalUI()

	for _, e := range listResult.Entries {
		if e.DirentType == keybase1.DirentType_DIR {
			ui.Printf("%s\t<%s>\t\t%s\n", keybase1.FormatTime(e.Time), keybase1.DirentTypeRevMap[e.DirentType], e.Name)
		} else {
			ui.Printf("%s\t%s\t%d\t%s\n", keybase1.FormatTime(e.Time), keybase1.DirentTypeRevMap[e.DirentType], e.Size, e.Name)
		}
	}
}

// ParseArgv gets the required path argument for this command.
func (c *CmdSimpleFSList) ParseArgv(ctx *cli.Context) error {
	nargs := len(ctx.Args())
	var err error

	c.recurse = ctx.Bool("recurse")

	if nargs < 1 {
		return errors.New("ls requires at least one KBFS path argument")
	}

	for _, src := range ctx.Args() {
		argPath := makeSimpleFSPath(c.G(), src)
		pathType, err := argPath.PathType()
		if err != nil {
			return err
		}
		if pathType != keybase1.PathType_KBFS {
			return errors.New("ls requires KBFS path arguments")
		}
		c.paths = append(c.paths, argPath)
	}

	return err
}

// GetUsage says what this command needs to operate.
func (c *CmdSimpleFSList) GetUsage() libkb.Usage {
	return libkb.Usage{
		Config:    true,
		KbKeyring: true,
		API:       true,
	}
}