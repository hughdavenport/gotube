package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"runtime"
	"syscall"
	"time"

	"github.com/hanwen/go-fuse/v2/fs"
	"github.com/hanwen/go-fuse/v2/fuse"

	"google.golang.org/api/youtube/v3"
)

const TIMEOUT = 1 * time.Minute

func Unimplemented() {
	callers := make([]uintptr, 1)
	runtime.Callers(2, callers)
	frames := runtime.CallersFrames(callers)
	frame, _ := frames.Next()
	fmt.Printf("%s:%d: Unimplemented %s\n", frame.File, frame.Line, frame.Function)
}

type GotubeOptions struct {
	YoutubeService *youtube.Service
	MountTime      time.Time
}
type Root struct {
	fs.Inode
	options GotubeOptions
}

var _ = (fs.NodeOnAdder)((*Root)(nil))
var _ = (fs.NodeGetattrer)((*Root)(nil))

func (r *Root) OnAdd(ctx context.Context) {
	studio := r.NewPersistentInode(ctx, &StudioRoot{options: r.options}, fs.StableAttr{Mode: fuse.S_IFDIR})
	r.AddChild("studio", studio, false)
}

func (r *Root) Getattr(ctx context.Context, fh fs.FileHandle, out *fuse.AttrOut) syscall.Errno {
	out.Mode = 0755
	out.SetTimes(&r.options.MountTime, &r.options.MountTime, &r.options.MountTime)
	return 0
}

func main() {
	debug := flag.Bool("debug", false, "print debug data")
	flag.Parse()
	if len(flag.Args()) < 1 {
		log.Fatal("Usage:\n  hello MOUNTPOINT")
	}
	mount_options := &fs.Options{}
	mount_options.Debug = *debug

	ctx := context.Background()
	service, err := YoutubeService(ctx)
	if err != nil {
		log.Panic("Could not get YouTube service")
	}
	log.Print("Got YouTube service connection")

	now := time.Now()
	gotube_options := GotubeOptions{
		YoutubeService: service,
		MountTime:      now,
	}
	server, err := fs.Mount(flag.Arg(0), &Root{options: gotube_options}, mount_options)
	if err != nil {
		log.Fatalf("Mount fail: %v\n", err)
	}

	sigchan := make(chan os.Signal)
	signal.Notify(sigchan, os.Interrupt)
	go func() {
		<-sigchan
		log.Print("Unmounted")
		server.Unmount()
	}()

	log.Print("Mounted")
	server.Wait()
}
