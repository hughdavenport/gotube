package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"time"
    "maps"
    "iter"

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

type Cache[K comparable, V any] struct {
	store map[K]V
    lock sync.RWMutex
}

func NewCache[K comparable, V any]() (Cache[K, V]) {
    return Cache[K, V]{
        store: make(map[K]V),
    }
}

func (c *Cache[_, _]) Clear() {
	clear(c.store)
}
func (c *Cache[K, V]) Store(key K, value V) {
	c.store[key] = value
	// log.Printf("CACHE %s = %s", key, value)
}
func (c *Cache[K, V]) Load(key K) (V, bool) {
	value, ok := c.store[key]
	if ok {
		// log.Printf("HIT %s = %s", key, value)
	} else {
		log.Printf("MISS %s", key)
	}
	return value, ok
}
func (c *Cache[_, _]) Len() (int) {
    return len(c.store)
}
func (c *Cache[K, _]) Entries() (iter.Seq[K]) {
    return maps.Keys(c.store)
}
func (c *Cache[_, _]) RLock() {
    c.lock.RLock()
}
func (c *Cache[_, _]) RUnlock() {
    c.lock.RUnlock()
}
func (c *Cache[_, _]) Lock() {
    c.lock.Lock()
}
func (c *Cache[_, _]) Unlock() {
    c.lock.Unlock()
}

type GotubeOptions struct {
	YoutubeService *youtube.Service
	MountTime      time.Time
}
type Root struct {
	fs.Inode
	Options GotubeOptions
}
type StudioRoot struct {
	fs.Inode
	Options GotubeOptions
}
type StudioVideosRoot struct {
	fs.Inode
	Options GotubeOptions
}
type StudioPlaylistsRoot struct {
	fs.Inode
	Options      GotubeOptions
	cache        Cache[string, *youtube.Playlist]
}
type StudioAnalyticsRoot struct {
	fs.Inode
	Options GotubeOptions
}
type StudioPlaylistNode struct {
	fs.Inode
	Playlist *youtube.Playlist
	Options  GotubeOptions
}

var _ = (fs.NodeOnAdder)((*Root)(nil))
var _ = (fs.NodeGetattrer)((*Root)(nil))

var _ = (fs.NodeOnAdder)((*StudioRoot)(nil))
var _ = (fs.NodeGetattrer)((*StudioRoot)(nil))

var _ = (fs.NodeLookuper)((*StudioVideosRoot)(nil))
var _ = (fs.NodeReaddirer)((*StudioVideosRoot)(nil))
var _ = (fs.NodeGetattrer)((*StudioVideosRoot)(nil))

var _ = (fs.NodeLookuper)((*StudioPlaylistsRoot)(nil))
var _ = (fs.NodeReaddirer)((*StudioPlaylistsRoot)(nil))
var _ = (fs.NodeGetattrer)((*StudioPlaylistsRoot)(nil))

var _ = (fs.NodeLookuper)((*StudioAnalyticsRoot)(nil))
var _ = (fs.NodeReaddirer)((*StudioAnalyticsRoot)(nil))
var _ = (fs.NodeGetattrer)((*StudioAnalyticsRoot)(nil))

var _ = (fs.NodeLookuper)((*StudioPlaylistNode)(nil))
var _ = (fs.NodeReaddirer)((*StudioPlaylistNode)(nil))
var _ = (fs.NodeGetattrer)((*StudioPlaylistNode)(nil))

func (r *Root) OnAdd(ctx context.Context) {
	studio := r.NewPersistentInode(ctx, &StudioRoot{Options: r.Options}, fs.StableAttr{Mode: fuse.S_IFDIR})
	r.AddChild("studio", studio, false)
}

func (r *Root) Getattr(ctx context.Context, fh fs.FileHandle, out *fuse.AttrOut) syscall.Errno {
	out.Mode = 0755
	out.SetTimes(&r.Options.MountTime, &r.Options.MountTime, &r.Options.MountTime)
	return 0
}

func (r *StudioRoot) OnAdd(ctx context.Context) {
    playlists := r.NewPersistentInode(ctx, &StudioPlaylistsRoot{Options: r.Options, cache: NewCache[string, *youtube.Playlist]()}, fs.StableAttr{Mode: fuse.S_IFDIR})
	videos := r.NewPersistentInode(ctx, &StudioVideosRoot{Options: r.Options}, fs.StableAttr{Mode: fuse.S_IFDIR})
	analytics := r.NewPersistentInode(ctx, &StudioAnalyticsRoot{Options: r.Options}, fs.StableAttr{Mode: fuse.S_IFDIR})
	r.AddChild("playlists", playlists, false)
	r.AddChild("videos", videos, false)
	r.AddChild("analytics", analytics, false)
}

func (r *StudioRoot) Getattr(ctx context.Context, fh fs.FileHandle, out *fuse.AttrOut) syscall.Errno {
	out.Mode = 0755
	out.SetTimes(&r.Options.MountTime, &r.Options.MountTime, &r.Options.MountTime)
	return 0
}

func (r *StudioPlaylistsRoot) refreshCache() syscall.Errno {
    log.Print("Refreshing Playlist cache")
    r.cache.Lock()
    defer log.Print("Refreshed")
    defer r.cache.Unlock()
    log.Print("locked")
	r.cache.Clear()
    log.Print("cleared")
	time.AfterFunc(TIMEOUT, func() {
        log.Print("Clearing Playlist cache")
        r.cache.Lock()
        defer log.Print("Cleared")
        defer r.cache.Unlock()
		r.cache.Clear()
	})
	call := r.Options.YoutubeService.Playlists.List([]string{"snippet", "status"})
	call = call.Mine(true)
	response, err := call.Do()
	if err != nil {
		log.Print("Unable to get list of playlists: %+v", err)
		return syscall.EAGAIN
	}
	total := response.PageInfo.TotalResults
	call = call.MaxResults(total)
	response, err = call.Do()
	if err != nil {
		log.Print("Unable to get list of playlists: %+v", err)
		return syscall.EAGAIN
	}
	for _, playlist := range response.Items {
		title := playlist.Snippet.Title
		if playlist.Status.PrivacyStatus != "public" {
			title = "." + title
		}
		title = strings.ReplaceAll(title, "/", "_")
		r.cache.Store(title, playlist)
	}
	return 0
}

func (r *StudioPlaylistsRoot) getPlaylist(name string) (*youtube.Playlist, syscall.Errno) {
    if r.cache.Len() == 0 {
        go r.refreshCache()
    }
    r.cache.RLock()
    defer r.cache.RUnlock()
	playlist, ok := r.cache.Load(name)
	if !ok {
		return nil, syscall.ENOENT
	}
	return playlist, 0
}

func (r *StudioPlaylistsRoot) Lookup(ctx context.Context, name string, out *fuse.EntryOut) (*fs.Inode, syscall.Errno) {
    if strings.HasPrefix(name, ".") {
        log.Printf("Playlists Lookup [PRIVATE]")
    } else {
        log.Printf("Playlists Lookup %s", name)
    }
	playlist, err := r.getPlaylist(name)
	if err != 0 {
		return nil, syscall.ENOENT
	}

	out.SetEntryTimeout(TIMEOUT)
	out.SetAttrTimeout(TIMEOUT)
	out.Mode = 0755
	now := time.Now()

	published_at, parse_err := time.Parse(time.RFC3339, playlist.Snippet.PublishedAt)
	if parse_err != nil {
		log.Printf("time = %s, err = %s", playlist.Snippet.PublishedAt, parse_err)
		published_at = time.UnixMilli(0)
	}

	atime := now
	mtime := published_at
	ctime := published_at
	out.SetTimes(&atime, &mtime, &ctime)
	return r.NewInode(ctx, &StudioPlaylistNode{Options: r.Options, Playlist: playlist}, fs.StableAttr{
		Mode: fuse.S_IFDIR,
	}), 0
}

func (r *StudioPlaylistsRoot) Readdir(ctx context.Context) (fs.DirStream, syscall.Errno) {
	if r.cache.Len() == 0 {
		r.refreshCache()
	}
    r.cache.RLock()
    defer r.cache.RUnlock()
	entries := make([]fuse.DirEntry, 0, r.cache.Len())
	for title := range r.cache.Entries() {
		entries = append(entries, fuse.DirEntry{
			Name: title,
			Mode: fuse.S_IFDIR,
		})
	}
	return fs.NewListDirStream(entries), 0
}

func (r *StudioPlaylistsRoot) Getattr(ctx context.Context, fh fs.FileHandle, out *fuse.AttrOut) syscall.Errno {
	out.Mode = 0755
	out.SetTimes(&r.Options.MountTime, &r.Options.MountTime, &r.Options.MountTime)
	return 0
}

func (r *StudioVideosRoot) Lookup(ctx context.Context, name string, out *fuse.EntryOut) (*fs.Inode, syscall.Errno) {
	Unimplemented()
	return nil, syscall.ENOSYS
}

func (r *StudioVideosRoot) Readdir(ctx context.Context) (fs.DirStream, syscall.Errno) {
	Unimplemented()
	return nil, syscall.ENOSYS
}

func (r *StudioVideosRoot) Getattr(ctx context.Context, fh fs.FileHandle, out *fuse.AttrOut) syscall.Errno {
	out.Mode = 0755
	out.SetTimes(&r.Options.MountTime, &r.Options.MountTime, &r.Options.MountTime)
	return 0
}

func (r *StudioAnalyticsRoot) Lookup(ctx context.Context, name string, out *fuse.EntryOut) (*fs.Inode, syscall.Errno) {
	Unimplemented()
	return nil, syscall.ENOSYS
}

func (r *StudioAnalyticsRoot) Readdir(ctx context.Context) (fs.DirStream, syscall.Errno) {
	Unimplemented()
	return nil, syscall.ENOSYS
}

func (r *StudioAnalyticsRoot) Getattr(ctx context.Context, fh fs.FileHandle, out *fuse.AttrOut) syscall.Errno {
	out.Mode = 0755
	out.SetTimes(&r.Options.MountTime, &r.Options.MountTime, &r.Options.MountTime)
	return 0
}

func (r *StudioPlaylistNode) Lookup(ctx context.Context, name string, out *fuse.EntryOut) (*fs.Inode, syscall.Errno) {
	Unimplemented()
	return nil, syscall.ENOSYS
}

func (r *StudioPlaylistNode) Readdir(ctx context.Context) (fs.DirStream, syscall.Errno) {
	Unimplemented()
	return nil, syscall.ENOSYS
}

func (r *StudioPlaylistNode) Getattr(ctx context.Context, fh fs.FileHandle, out *fuse.AttrOut) syscall.Errno {
	out.Mode = 0755
	now := time.Now()

	published_at, parse_err := time.Parse(time.RFC3339, r.Playlist.Snippet.PublishedAt)
	if parse_err != nil {
		log.Printf("time = %s, err = %s", r.Playlist.Snippet.PublishedAt, parse_err)
		published_at = time.UnixMilli(0)
	}

	atime := now
	mtime := published_at
	ctime := published_at
	out.SetTimes(&atime, &mtime, &ctime)
	out.SetTimeout(TIMEOUT)
	log.Printf("getattr on %s", r.Playlist.Snippet.Title)
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
	server, err := fs.Mount(flag.Arg(0), &Root{Options: gotube_options}, mount_options)
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
