package main

import (
	"context"
	"log"
	"strings"
	"syscall"
	"time"

	"github.com/hanwen/go-fuse/v2/fs"
	"github.com/hanwen/go-fuse/v2/fuse"

	"google.golang.org/api/youtube/v3"
)

type StudioPlaylistsRoot struct {
	fs.Inode
	options GotubeOptions
	cache   Cache[string, *youtube.Playlist]
}

type StudioPlaylistNode struct {
	fs.Inode
	id      string
	time    time.Time
	title   string
	options GotubeOptions
	cache   Cache[string, *youtube.PlaylistItem]
}

var _ = (fs.NodeLookuper)((*StudioPlaylistsRoot)(nil))
var _ = (fs.NodeReaddirer)((*StudioPlaylistsRoot)(nil))
var _ = (fs.NodeGetattrer)((*StudioPlaylistsRoot)(nil))

var _ = (fs.NodeLookuper)((*StudioPlaylistNode)(nil))
var _ = (fs.NodeReaddirer)((*StudioPlaylistNode)(nil))
var _ = (fs.NodeGetattrer)((*StudioPlaylistNode)(nil))

func NewStudioPlaylistRoot(options GotubeOptions) *StudioPlaylistsRoot {
	return &StudioPlaylistsRoot{
		options: options,
		cache:   NewCache[string, *youtube.Playlist](),
	}
}

func (r *StudioPlaylistsRoot) refreshCache() syscall.Errno {
	log.Print("Refreshing Playlist cache")

	r.cache.Lock()
	defer r.cache.Unlock()
	r.cache.Clear()

	time.AfterFunc(TIMEOUT, func() {
		r.cache.Lock()
		defer r.cache.Unlock()
		r.cache.Clear()
	})

	call := r.options.YoutubeService.Playlists.List([]string{"snippet", "status"})
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
	r.cache.RLock()
	defer r.cache.RUnlock()
	if r.cache.Len() == 0 {
		go r.refreshCache()
	}

	playlist, ok := r.cache.Load(name)
	if !ok {
		return nil, syscall.ENOENT
	}
	return playlist, 0
}

func (r *StudioPlaylistsRoot) Lookup(ctx context.Context, name string, out *fuse.EntryOut) (*fs.Inode, syscall.Errno) {
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
		log.Printf("Couldn't parse time %s, err = %s", playlist.Snippet.PublishedAt, parse_err)
		published_at = time.UnixMilli(0)
	}

	atime := now
	mtime := published_at
	ctime := published_at
	out.SetTimes(&atime, &mtime, &ctime)

	return r.NewInode(ctx, &StudioPlaylistNode{
		options: r.options,
		id:      playlist.Id,
		time:    published_at,
		title:   name,
		cache:   NewCache[string, *youtube.PlaylistItem]()},
		fs.StableAttr{
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
	out.SetTimes(&r.options.MountTime, &r.options.MountTime, &r.options.MountTime)
	return 0
}

func (r *StudioPlaylistNode) refreshCache() syscall.Errno {
	log.Printf("Refreshing Playlist cache for %s", r.title)

	r.cache.Lock()
	defer r.cache.Unlock()
	r.cache.Clear()

	time.AfterFunc(TIMEOUT, func() {
		r.cache.Lock()
		defer r.cache.Unlock()
		r.cache.Clear()
	})

	call := r.options.YoutubeService.PlaylistItems.List([]string{"snippet", "status"})
	call = call.PlaylistId(r.id)
	response, err := call.Do()
	if err != nil {
		log.Print("Unable to get list of playlist %s: %+v", r.title, err)
		return syscall.EAGAIN
	}

	total := response.PageInfo.TotalResults
	call = call.MaxResults(total)
	response, err = call.Do()
	if err != nil {
		log.Print("Unable to get list of playlist %s: %+v", r.title, err)
		return syscall.EAGAIN
	}

	for _, item := range response.Items {
		title := item.Snippet.Title
		if item.Status.PrivacyStatus != "public" {
			title = "." + title
		}
		title = strings.ReplaceAll(title, "/", "_")
		r.cache.Store(title, item)
	}

	return 0
}

func (r *StudioPlaylistNode) Lookup(ctx context.Context, name string, out *fuse.EntryOut) (*fs.Inode, syscall.Errno) {
	Unimplemented()
	return nil, syscall.ENOSYS
}

func (r *StudioPlaylistNode) Readdir(ctx context.Context) (fs.DirStream, syscall.Errno) {
	if r.cache.Len() == 0 {
		r.refreshCache()
	}
	r.cache.RLock()
	defer r.cache.RUnlock()

	entries := make([]fuse.DirEntry, 0, r.cache.Len())
	for title := range r.cache.Entries() {
		entries = append(entries, fuse.DirEntry{
			Name: title,
			Mode: fuse.S_IFLNK,
		})
	}

	return fs.NewListDirStream(entries), 0
}

func (r *StudioPlaylistNode) Getattr(ctx context.Context, fh fs.FileHandle, out *fuse.AttrOut) syscall.Errno {
	out.Mode = 0755
	now := time.Now()

	atime := now
	mtime := r.time
	ctime := r.time
	out.SetTimes(&atime, &mtime, &ctime)
	out.SetTimeout(TIMEOUT)
	log.Printf("getattr on %s", r.title)
	return 0
}
