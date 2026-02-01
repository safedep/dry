package artifactv2

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"path"
	"sync"
	"time"

	"github.com/safedep/dry/log"
	"github.com/safedep/dry/storage"
)

// archiveType represents the type of archive format
type archiveType string

const (
	archiveTypeTarGz archiveType = "tar.gz"
	archiveTypeZip   archiveType = "zip"
	archiveTypeTar   archiveType = "tar"
)

// archiveEntryInfo holds cached information about an archive entry
type archiveEntryInfo struct {
	path    string
	size    int64
	modTime time.Time
	isDir   bool

	// Position in the archive stream (for future seeking optimizations)
	offset int64
}

// archiveIndexCache holds an indexed map of archive entries for fast lookups
type archiveIndexCache struct {
	// path -> entry info
	entries map[string]*archiveEntryInfo
	indexed time.Time
}

// archiveReader provides a unified interface for reading different archive formats
type archiveReader struct {
	artifactID     string
	storage        StorageManager
	archiveType    archiveType
	indexCache     *archiveIndexCache
	indexCacheLock sync.Mutex
}

// newArchiveReader creates a new archive reader with lazy index caching
func newArchiveReader(artifactID string, storage StorageManager, archiveType archiveType) *archiveReader {
	return &archiveReader{
		artifactID:  artifactID,
		storage:     storage,
		archiveType: archiveType,
	}
}

// openTarGzReader creates and returns a tar.gz reader along with the underlying readers
// that need to be closed. Callers must close the returned readers in reverse order.
func openTarGzReader(artifactReader io.ReadCloser) (*tar.Reader, *gzip.Reader, error) {
	gzipReader, err := gzip.NewReader(artifactReader)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create gzip reader: %w", err)
	}

	tarReader := tar.NewReader(gzipReader)
	return tarReader, gzipReader, nil
}

// withTarGzReader executes a function with an open tar.gz reader, handling cleanup automatically
func (r *archiveReader) withTarGzReader(ctx context.Context, fn func(*tar.Reader) error) error {
	artifactReader, err := r.storage.Get(ctx, r.artifactID)
	if err != nil {
		return fmt.Errorf("failed to get artifact reader: %w", err)
	}

	defer artifactReader.Close()

	tarReader, gzipReader, err := openTarGzReader(artifactReader)
	if err != nil {
		return err
	}

	defer gzipReader.Close()

	return fn(tarReader)
}

// ensureIndexed builds the archive index cache if it hasn't been built yet
func (r *archiveReader) ensureIndexed(ctx context.Context) error {
	r.indexCacheLock.Lock()
	defer r.indexCacheLock.Unlock()

	if r.indexCache != nil {
		return nil
	}

	switch r.archiveType {
	case archiveTypeTarGz:
		return r.buildTarGzIndex(ctx)
	default:
		return fmt.Errorf("unsupported archive type: %s", r.archiveType)
	}
}

// buildTarGzIndex builds an index for tar.gz archives
func (r *archiveReader) buildTarGzIndex(ctx context.Context) error {
	cache := &archiveIndexCache{
		entries: make(map[string]*archiveEntryInfo),
		indexed: time.Now(),
	}

	err := r.withTarGzReader(ctx, func(tarReader *tar.Reader) error {
		offset := int64(0)
		for {
			header, err := tarReader.Next()
			if err == io.EOF {
				break
			}

			if err != nil {
				return fmt.Errorf("failed to read tar header: %w", err)
			}

			cache.entries[header.Name] = &archiveEntryInfo{
				path:    header.Name,
				size:    header.Size,
				modTime: header.ModTime,
				isDir:   header.Typeflag == tar.TypeDir,
				offset:  offset,
			}

			offset += header.Size
		}

		return nil
	})

	if err != nil {
		return fmt.Errorf("failed to build tar.gz index: %w", err)
	}

	r.indexCache = cache
	log.Debugf("Built archive index for artifact %s: %d entries", r.artifactID, len(cache.entries))

	return nil
}

// getEntry retrieves a cached archive entry by path
func (r *archiveReader) getEntry(ctx context.Context, path string) (*archiveEntryInfo, error) {
	if err := r.ensureIndexed(ctx); err != nil {
		return nil, err
	}

	r.indexCacheLock.Lock()
	entry, exists := r.indexCache.entries[path]
	r.indexCacheLock.Unlock()

	if !exists {
		return nil, fmt.Errorf("entry not found: %s", path)
	}

	return entry, nil
}

// listEntries returns a list of all entry paths in the archive
func (r *archiveReader) listEntries(ctx context.Context, filesOnly bool) ([]string, error) {
	if err := r.ensureIndexed(ctx); err != nil {
		return nil, err
	}

	r.indexCacheLock.Lock()
	defer r.indexCacheLock.Unlock()

	entries := make([]string, 0, len(r.indexCache.entries))
	for path, entry := range r.indexCache.entries {
		if filesOnly && entry.isDir {
			continue
		}

		entries = append(entries, path)
	}

	return entries, nil
}

// enumFiles enumerates all files (not directories) in the archive
func (r *archiveReader) enumFiles(ctx context.Context, fn func(FileInfo) error) error {
	if r.archiveType != archiveTypeTarGz {
		return fmt.Errorf("unsupported archive type for enumeration: %s", r.archiveType)
	}

	return r.withTarGzReader(ctx, func(tarReader *tar.Reader) error {
		for {
			header, err := tarReader.Next()
			if err == io.EOF {
				break
			}
			if err != nil {
				return fmt.Errorf("failed to read tar header: %w", err)
			}

			// Skip directories
			if header.Typeflag != tar.TypeReg {
				continue
			}

			fileInfo := FileInfo{
				Path:    header.Name,
				Size:    header.Size,
				ModTime: header.ModTime,
				IsDir:   false,
				Reader:  tarReader,
			}

			if err := fn(fileInfo); err != nil {
				return err
			}
		}

		return nil
	})
}

// readFile reads a specific file from the archive
// Returns a reader that must be closed by the caller
func (r *archiveReader) readFile(ctx context.Context, path string) (io.ReadCloser, error) {
	entry, err := r.getEntry(ctx, path)
	if err != nil {
		return nil, err
	}

	if entry.isDir {
		return nil, fmt.Errorf("cannot read directory: %s", path)
	}

	if r.archiveType != archiveTypeTarGz {
		return nil, fmt.Errorf("unsupported archive type for file reading: %s", r.archiveType)
	}

	artifactReader, err := r.storage.Get(ctx, r.artifactID)
	if err != nil {
		return nil, fmt.Errorf("failed to get artifact reader: %w", err)
	}

	tarReader, gzipReader, err := openTarGzReader(artifactReader)
	if err != nil {
		artifactReader.Close()
		return nil, err
	}

	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			gzipReader.Close()
			artifactReader.Close()
			return nil, fmt.Errorf("file not found: %s", path)
		}

		if err != nil {
			gzipReader.Close()
			artifactReader.Close()
			return nil, fmt.Errorf("failed to read tar header: %w", err)
		}

		if header.Name == path {
			return &tarFileReader{
				tarReader:  tarReader,
				gzipReader: gzipReader,
				fileReader: artifactReader,
			}, nil
		}
	}
}

// tarFileReader wraps tar reading with proper cleanup of all layers
type tarFileReader struct {
	tarReader  *tar.Reader
	gzipReader *gzip.Reader
	fileReader io.ReadCloser
}

func (r *tarFileReader) Read(p []byte) (n int, err error) {
	return r.tarReader.Read(p)
}

func (r *tarFileReader) Close() error {
	var errs []error

	if err := r.gzipReader.Close(); err != nil {
		errs = append(errs, fmt.Errorf("gzip close: %w", err))
	}

	if err := r.fileReader.Close(); err != nil {
		errs = append(errs, fmt.Errorf("file close: %w", err))
	}

	if len(errs) > 0 {
		return fmt.Errorf("close errors: %v", errs)
	}

	return nil
}

// extractToStorage extracts archive contents to storage
// Returns information about the extraction including file count and total size
func extractToStorage(
	ctx context.Context,
	store storage.Storage,
	archiveReader *archiveReader,
	baseKey string,
) (*ExtractResult, error) {
	existingFiles, err := store.List(ctx, baseKey)
	if err == nil && len(existingFiles) > 0 {
		// Already extracted, compute metrics from existing files
		log.Debugf("Archive already extracted to %s (%d files)", baseKey, len(existingFiles))

		// Build result from existing extraction
		totalSize := int64(0)
		fileCount := len(existingFiles)

		return &ExtractResult{
			ExtractionKey:    baseKey,
			FileCount:        fileCount,
			TotalSize:        totalSize,
			AlreadyExtracted: true,
		}, nil
	}

	log.Debugf("Extracting archive to %s", baseKey)

	fileCount := 0
	totalSize := int64(0)
	extractionErrors := make([]error, 0)

	err = archiveReader.enumFiles(ctx, func(fileInfo FileInfo) error {
		if fileInfo.IsDir {
			return nil
		}

		fileKey := path.Join(baseKey, fileInfo.Path)

		// Stream file content directly to storage using LimitReader to avoid memory buffering
		// LimitReader ensures we only read fileInfo.Size bytes from the tar stream
		limitedReader := io.LimitReader(fileInfo.Reader, fileInfo.Size)

		if err := store.Put(fileKey, limitedReader); err != nil {
			extractionErrors = append(extractionErrors,
				fmt.Errorf("failed to write %s: %w", fileInfo.Path, err))

			return nil
		}

		fileCount++
		totalSize += fileInfo.Size

		log.Debugf("Extracted %s (%d bytes)", fileInfo.Path, fileInfo.Size)

		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to enumerate archive files: %w", err)
	}

	if len(extractionErrors) > 0 {
		log.Warnf("Extraction completed with %d errors", len(extractionErrors))
	}

	log.Debugf("Extraction complete: %d files, %d bytes", fileCount, totalSize)

	return &ExtractResult{
		ExtractionKey:    baseKey,
		FileCount:        fileCount,
		TotalSize:        totalSize,
		AlreadyExtracted: false,
	}, nil
}
