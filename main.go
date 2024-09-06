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

	"github.com/hanwen/go-fuse/v2/fs"
	"github.com/hanwen/go-fuse/v2/fuse"

	"google.golang.org/api/youtube/v3"
)

func Unimplemented() {
	callers := make([]uintptr, 1)
	runtime.Callers(2, callers)
	frames := runtime.CallersFrames(callers)
	frame, _ := frames.Next()
	fmt.Printf("%s:%d: Unimplemented %s", frame.File, frame.Line, frame.Function)
}

type Root struct{ fs.Inode }
type StudioRoot struct {
	fs.Inode
	*youtube.Service
}
type StudioVideosRoot struct {
	fs.Inode
	*youtube.Service
}
type StudioPlaylistsRoot struct {
	fs.Inode
	*youtube.Service
}
type StudioAnalyticsRoot struct {
	fs.Inode
	*youtube.Service
}

type key int

var youtubeKey key

func (r *Root) OnAdd(ctx context.Context) {
	service, err := YoutubeService(ctx)
	if err != nil {
		log.Panic("Could not get YouTube service")
	}
	log.Print("Got YouTube service connection")
	studio := r.NewPersistentInode(ctx, &StudioRoot{Service: service}, fs.StableAttr{Mode: fuse.S_IFDIR})
	r.AddChild("studio", studio, false)
}

func (r *Root) Getattr(ctx context.Context, fh fs.FileHandle, out *fuse.AttrOut) syscall.Errno {
	out.Mode = 0755
	return 0
}

func (r *StudioRoot) OnAdd(ctx context.Context) {
	playlists := r.NewPersistentInode(ctx, &StudioPlaylistsRoot{Service: r.Service}, fs.StableAttr{Mode: fuse.S_IFDIR})
	videos := r.NewPersistentInode(ctx, &StudioVideosRoot{Service: r.Service}, fs.StableAttr{Mode: fuse.S_IFDIR})
	analytics := r.NewPersistentInode(ctx, &StudioAnalyticsRoot{Service: r.Service}, fs.StableAttr{Mode: fuse.S_IFDIR})
	r.AddChild("playlists", playlists, false)
	r.AddChild("videos", videos, false)
	r.AddChild("analytics", analytics, false)
}

func (r *StudioRoot) Getattr(ctx context.Context, fh fs.FileHandle, out *fuse.AttrOut) syscall.Errno {
	out.Mode = 0755
	return 0
}

func (r *StudioPlaylistsRoot) Lookup(ctx context.Context, name string, out *fuse.EntryOut) (*fs.Inode, syscall.Errno) {
	Unimplemented()
	return nil, syscall.ENOSYS
}

func (r *StudioPlaylistsRoot) Readdir(ctx context.Context) (fs.DirStream, syscall.Errno) {
	call := r.Service.Playlists.List([]string{"snippet", "status"})
	call = call.Mine(true)
	response, err := call.Do()
	if err != nil {
		log.Print("Unable to get list of playlists: %+v", err)
		return nil, syscall.EAGAIN
	}
    total := response.PageInfo.TotalResults
    entries := make([]fuse.DirEntry, 0, total)
    call = call.MaxResults(total)
    response, err = call.Do()
    if err != nil {
        log.Print("Unable to get list of playlists: %+v", err)
        return nil, syscall.EAGAIN
    }
    for _, playlist := range response.Items {
        name := playlist.Snippet.Title
        if playlist.Status.PrivacyStatus != "public" {
            name = "." + name
        }
        entries = append(entries, fuse.DirEntry{
            Name: name,
            Mode: fuse.S_IFDIR,
        })
    }
	return fs.NewListDirStream(entries), 0
}

func (r *StudioVideosRoot) Lookup(ctx context.Context, name string, out *fuse.EntryOut) (*fs.Inode, syscall.Errno) {
	Unimplemented()
	return nil, syscall.ENOSYS
}

func (r *StudioVideosRoot) Readdir(ctx context.Context) (fs.DirStream, syscall.Errno) {
	Unimplemented()
	return nil, syscall.ENOSYS
}

func (r *StudioAnalyticsRoot) Lookup(ctx context.Context, name string, out *fuse.EntryOut) (*fs.Inode, syscall.Errno) {
	Unimplemented()
	return nil, syscall.ENOSYS
}

func (r *StudioAnalyticsRoot) Readdir(ctx context.Context) (fs.DirStream, syscall.Errno) {
	Unimplemented()
	return nil, syscall.ENOSYS
}

var _ = (fs.NodeOnAdder)((*Root)(nil))
var _ = (fs.NodeOnAdder)((*StudioRoot)(nil))

var _ = (fs.NodeLookuper)((*StudioVideosRoot)(nil))
var _ = (fs.NodeReaddirer)((*StudioVideosRoot)(nil))
var _ = (fs.NodeLookuper)((*StudioPlaylistsRoot)(nil))
var _ = (fs.NodeReaddirer)((*StudioPlaylistsRoot)(nil))
var _ = (fs.NodeLookuper)((*StudioAnalyticsRoot)(nil))
var _ = (fs.NodeReaddirer)((*StudioAnalyticsRoot)(nil))

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
