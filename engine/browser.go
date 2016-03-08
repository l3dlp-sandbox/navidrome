package engine

import (
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/astaxie/beego"
	"github.com/deluan/gosonic/consts"
	"github.com/deluan/gosonic/domain"
	"github.com/deluan/gosonic/utils"
)

var (
	DataNotFound = errors.New("Data Not Found")
)

type Browser interface {
	MediaFolders() (domain.MediaFolders, error)
	Indexes(ifModifiedSince time.Time) (domain.ArtistIndexes, time.Time, error)
	Directory(id string) (*DirectoryInfo, error)
}

func NewBrowser(pr domain.PropertyRepository, fr domain.MediaFolderRepository, ir domain.ArtistIndexRepository,
	ar domain.ArtistRepository, alr domain.AlbumRepository, mr domain.MediaFileRepository) Browser {
	return browser{pr, fr, ir, ar, alr, mr}
}

type browser struct {
	propRepo   domain.PropertyRepository
	folderRepo domain.MediaFolderRepository
	indexRepo  domain.ArtistIndexRepository
	artistRepo domain.ArtistRepository
	albumRepo  domain.AlbumRepository
	mfileRepo  domain.MediaFileRepository
}

func (b browser) MediaFolders() (domain.MediaFolders, error) {
	return b.folderRepo.GetAll()
}

func (b browser) Indexes(ifModifiedSince time.Time) (domain.ArtistIndexes, time.Time, error) {
	l, err := b.propRepo.DefaultGet(consts.LastScan, "-1")
	ms, _ := strconv.ParseInt(l, 10, 64)
	lastModified := utils.ToTime(ms)

	if err != nil {
		return domain.ArtistIndexes{}, time.Time{}, errors.New(fmt.Sprintf("Error retrieving LastScan property: %v", err))
	}

	if lastModified.After(ifModifiedSince) {
		indexes, err := b.indexRepo.GetAll()
		return indexes, lastModified, err
	}

	return domain.ArtistIndexes{}, lastModified, nil
}

type Child struct {
	Id          string
	Title       string
	IsDir       bool
	Parent      string
	Album       string
	Year        int
	Artist      string
	Genre       string
	CoverArt    string
	Starred     time.Time
	Track       int
	Duration    int
	Size        string
	Suffix      string
	BitRate     int
	ContentType string
}

type DirectoryInfo struct {
	Id       string
	Name     string
	Children []Child
}

func (c browser) Directory(id string) (*DirectoryInfo, error) {
	var dir *DirectoryInfo
	switch {
	case c.isArtist(id):
		beego.Info("Found Artist with id", id)
		a, albums, err := c.retrieveArtist(id)
		if err != nil {
			return nil, err
		}
		dir = c.buildArtistDir(a, albums)
	case c.isAlbum(id):
		beego.Info("Found Album with id", id)
		al, tracks, err := c.retrieveAlbum(id)
		if err != nil {
			return nil, err
		}
		dir = c.buildAlbumDir(al, tracks)
	default:
		beego.Info("Id", id, "not found")
		return nil, DataNotFound
	}

	return dir, nil
}

func (c browser) buildArtistDir(a *domain.Artist, albums []domain.Album) *DirectoryInfo {
	dir := &DirectoryInfo{Id: a.Id, Name: a.Name}

	dir.Children = make([]Child, len(albums))
	for i, al := range albums {
		dir.Children[i].Id = al.Id
		dir.Children[i].Title = al.Name
		dir.Children[i].IsDir = true
		dir.Children[i].Parent = al.ArtistId
		dir.Children[i].Album = al.Name
		dir.Children[i].Year = al.Year
		dir.Children[i].Artist = al.AlbumArtist
		dir.Children[i].Genre = al.Genre
		dir.Children[i].CoverArt = al.CoverArtId
		if al.Starred {
			dir.Children[i].Starred = al.UpdatedAt
		}

	}
	return dir
}

func (c browser) buildAlbumDir(al *domain.Album, tracks []domain.MediaFile) *DirectoryInfo {
	dir := &DirectoryInfo{Id: al.Id, Name: al.Name}

	dir.Children = make([]Child, len(tracks))
	for i, mf := range tracks {
		dir.Children[i].Id = mf.Id
		dir.Children[i].Title = mf.Title
		dir.Children[i].IsDir = false
		dir.Children[i].Parent = mf.AlbumId
		dir.Children[i].Album = mf.Album
		dir.Children[i].Year = mf.Year
		dir.Children[i].Artist = mf.Artist
		dir.Children[i].Genre = mf.Genre
		dir.Children[i].Track = mf.TrackNumber
		dir.Children[i].Duration = mf.Duration
		dir.Children[i].Size = mf.Size
		dir.Children[i].Suffix = mf.Suffix
		dir.Children[i].BitRate = mf.BitRate
		if mf.Starred {
			dir.Children[i].Starred = mf.UpdatedAt
		}
		if mf.HasCoverArt {
			dir.Children[i].CoverArt = mf.Id
		}
		dir.Children[i].ContentType = mf.ContentType()
	}
	return dir
}

func (c browser) isArtist(id string) bool {
	found, err := c.artistRepo.Exists(id)
	if err != nil {
		beego.Error(fmt.Errorf("Error searching for Artist %s: %v", id, err))
		return false
	}
	return found
}

func (c browser) isAlbum(id string) bool {
	found, err := c.albumRepo.Exists(id)
	if err != nil {
		beego.Error(fmt.Errorf("Error searching for Album %s: %v", id, err))
		return false
	}
	return found
}

func (c browser) retrieveArtist(id string) (a *domain.Artist, as []domain.Album, err error) {
	a, err = c.artistRepo.Get(id)
	if err != nil {
		err = fmt.Errorf("Error reading Artist %s from DB: %v", id, err)
		return
	}

	if as, err = c.albumRepo.FindByArtist(id); err != nil {
		err = fmt.Errorf("Error reading %s's albums from DB: %v", a.Name, err)
	}
	return
}

func (c browser) retrieveAlbum(id string) (al *domain.Album, mfs []domain.MediaFile, err error) {
	al, err = c.albumRepo.Get(id)
	if err != nil {
		err = fmt.Errorf("Error reading Album %s from DB: %v", id, err)
		return
	}

	if mfs, err = c.mfileRepo.FindByAlbum(id); err != nil {
		err = fmt.Errorf("Error reading %s's tracks from DB: %v", al.Name, err)
	}
	return
}
