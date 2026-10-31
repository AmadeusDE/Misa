package lscore

import (
	"strconv"
	"time"
)

// unavailable is what a listing prints for a field the platform cannot report.
const unavailable = "?"

// Fields names the metadata a platform provider may be unable to report.
// FileInfo.Missing holds the ones it could not; the zero value means all present.
type Fields uint16

const (
	FieldOwner Fields = 1 << iota
	FieldGroup
	FieldNlink
	FieldInode
	FieldDev
	FieldBlocks
	FieldAccessTime
	FieldChangeTime
	FieldBirthTime
)

const allFields = FieldOwner | FieldGroup | FieldNlink | FieldInode | FieldDev | FieldBlocks |
	FieldAccessTime | FieldChangeTime | FieldBirthTime

// sysMeta is what a platform provider reads beyond os.FileInfo.
type sysMeta struct {
	Missing    Fields
	UID, GID   uint32
	Owner      string
	Group      string
	Inode, Dev uint64
	Nlink      uint64
	Blocks     int64
	Access     time.Time
	Change     time.Time
	Birth      time.Time
}

func (fi *FileInfo) lacks(f Fields) bool { return fi.Missing&f != 0 }

func (fi *FileInfo) applyMeta(m sysMeta) {
	fi.Missing = m.Missing
	fi.UID, fi.GID = m.UID, m.GID
	fi.Owner, fi.Group = m.Owner, m.Group
	fi.Inode, fi.Dev, fi.Nlink, fi.Blocks = m.Inode, m.Dev, m.Nlink, m.Blocks
	fi.AccessTime, fi.ChangeTime, fi.BirthTime = m.Access, m.Change, m.Birth
}

func nlinkCell(fi *FileInfo) string {
	if fi.lacks(FieldNlink) {
		return unavailable
	}
	return strconv.FormatUint(fi.Nlink, 10)
}

func uidCell(fi *FileInfo) string {
	if fi.lacks(FieldOwner) {
		return unavailable
	}
	return strconv.FormatUint(uint64(fi.UID), 10)
}

func gidCell(fi *FileInfo) string {
	if fi.lacks(FieldGroup) {
		return unavailable
	}
	return strconv.FormatUint(uint64(fi.GID), 10)
}

func ownerCell(fi *FileInfo) string {
	if fi.lacks(FieldOwner) {
		return unavailable
	}
	return fi.Owner
}

func groupCell(fi *FileInfo) string {
	if fi.lacks(FieldGroup) {
		return unavailable
	}
	return fi.Group
}
