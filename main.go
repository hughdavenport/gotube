package main

import (
	"context"
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/hanwen/go-fuse/v2/fs"
	"github.com/hanwen/go-fuse/v2/fuse"

	"google.golang.org/api/youtube/v3"
)

type Root struct{ fs.Inode }
type StudioRoot struct{ fs.Inode }
type StudioVideosRoot struct{ fs.Inode }
type StudioPlaylistsRoot struct{ fs.Inode }
type StudioAnalyticsRoot struct{ fs.Inode }

type key int

var youtubeKey key

func (r *Root) OnAdd(ctx context.Context) {
	service, err := YoutubeService(ctx)
	if err != nil {
		log.Panic("Could not get YouTube service")
	}
    log.Print("Got YouTube service connection")
	youtube_ctx := context.WithValue(ctx, youtubeKey, service)
	studio := r.NewPersistentInode(youtube_ctx, &StudioRoot{}, fs.StableAttr{Mode: fuse.S_IFDIR})
	r.AddChild("studio", studio, false)
}

func (r *Root) Getattr(ctx context.Context, fh fs.FileHandle, out *fuse.AttrOut) syscall.Errno {
	out.Mode = 0755
	return 0
}

func (r *StudioRoot) OnAdd(ctx context.Context) {
	playlists := r.NewPersistentInode(ctx, &StudioPlaylistsRoot{}, fs.StableAttr{Mode: fuse.S_IFDIR})
	videos := r.NewPersistentInode(ctx, &StudioVideosRoot{}, fs.StableAttr{Mode: fuse.S_IFDIR})
	analytics := r.NewPersistentInode(ctx, &StudioAnalyticsRoot{}, fs.StableAttr{Mode: fuse.S_IFDIR})
	r.AddChild("playlists", playlists, false)
	r.AddChild("videos", videos, false)
	r.AddChild("analytics", analytics, false)
}

func (r *StudioRoot) Getattr(ctx context.Context, fh fs.FileHandle, out *fuse.AttrOut) syscall.Errno {
	out.Mode = 0755
	return 0
}

func (r *StudioPlaylistsRoot) OnAdd(ctx context.Context) {
	service, ok := ctx.Value(youtubeKey).(*youtube.Service)
	if !ok {
		log.Panic("Couldn't find context value")
	}
	log.Printf("context value %+v", service)
}

func main() {
	debug := flag.Bool("debug", false, "print debug data")
	flag.Parse()
	if len(flag.Args()) < 1 {
		log.Fatal("Usage:\n  hello MOUNTPOINT")
	}
	opts := &fs.Options{}
	opts.Debug = *debug
	server, err := fs.Mount(flag.Arg(0), &Root{}, opts)
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
