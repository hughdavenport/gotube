package main

import (
	"context"
	"syscall"

	"github.com/hanwen/go-fuse/v2/fs"
	"github.com/hanwen/go-fuse/v2/fuse"
)

type StudioRoot struct {
	fs.Inode
	options GotubeOptions
}

type StudioVideosRoot struct {
	fs.Inode
	options GotubeOptions
}

type StudioAnalyticsRoot struct {
	fs.Inode
	options GotubeOptions
}

var _ = (fs.NodeOnAdder)((*StudioRoot)(nil))
var _ = (fs.NodeGetattrer)((*StudioRoot)(nil))

var _ = (fs.NodeLookuper)((*StudioVideosRoot)(nil))
var _ = (fs.NodeReaddirer)((*StudioVideosRoot)(nil))
var _ = (fs.NodeGetattrer)((*StudioVideosRoot)(nil))

var _ = (fs.NodeLookuper)((*StudioAnalyticsRoot)(nil))
var _ = (fs.NodeReaddirer)((*StudioAnalyticsRoot)(nil))
var _ = (fs.NodeGetattrer)((*StudioAnalyticsRoot)(nil))

func (r *StudioRoot) OnAdd(ctx context.Context) {
	playlists := r.NewPersistentInode(ctx, NewStudioPlaylistRoot(r.options), fs.StableAttr{Mode: fuse.S_IFDIR})
	videos := r.NewPersistentInode(ctx, &StudioVideosRoot{options: r.options}, fs.StableAttr{Mode: fuse.S_IFDIR})
	analytics := r.NewPersistentInode(ctx, &StudioAnalyticsRoot{options: r.options}, fs.StableAttr{Mode: fuse.S_IFDIR})
	r.AddChild("playlists", playlists, false)
	r.AddChild("videos", videos, false)
	r.AddChild("analytics", analytics, false)
}

func (r *StudioRoot) Getattr(ctx context.Context, fh fs.FileHandle, out *fuse.AttrOut) syscall.Errno {
	out.Mode = 0755
	out.SetTimes(&r.options.MountTime, &r.options.MountTime, &r.options.MountTime)
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
	out.SetTimes(&r.options.MountTime, &r.options.MountTime, &r.options.MountTime)
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
	out.SetTimes(&r.options.MountTime, &r.options.MountTime, &r.options.MountTime)
	return 0
}
