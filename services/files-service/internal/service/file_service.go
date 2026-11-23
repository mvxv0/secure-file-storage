package service

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/mvxv0/secure-file-storage/services/files-service/internal/client"
	"github.com/mvxv0/secure-file-storage/services/files-service/internal/model"
	"github.com/mvxv0/secure-file-storage/services/files-service/internal/repository"
	"github.com/mvxv0/secure-file-storage/services/files-service/internal/storage"
)

var (
	ErrInvalidFileName         = errors.New("naziv datoteke mora imati izmedju 1 i 150 karaktera i ne sme sadrzati znakove: /, \\")
	ErrEmptyFileContent        = errors.New("datoteka ne moze biti prazna (0 bajtova)")
	ErrDisallowedFileExt       = errors.New("tip datoteke nije dozvoljen (whitelisting zastita)")
	ErrFileNotOwned            = errors.New("nemate pravo pristupa ili izmene ove datoteke")
	ErrQuotaExceeded           = errors.New("prekoracena kvota skladista: nemate dovoljno slobodnog prostora")
	ErrUsersServiceUnavailable = errors.New("users-service trenutno nije dostupan za proveru kvote")
)

// Whitelist dozvoljenih ekstenzija datoteka  (provera tipa datoteke / whitelisting)
var allowedExtensions = map[string]bool{
	".txt":  true,
	".md":   true,
	".csv":  true,
	".json": true,
	".xml":  true,
	".pdf":  true,
	".png":  true,
	".jpg":  true,
	".jpeg": true,
	".gif":  true,
	".webp": true,
	".svg":  true,
	".doc":  true,
	".docx": true,
	".xls":  true,
	".xlsx": true,
	".ppt":  true,
	".pptx": true,
	".zip":  true,
}

type FileService struct {
	fileRepo     *repository.FileRepository
	folderRepo   *repository.FolderRepository
	store        storage.ObjectStore
	quotaClient  client.QuotaClient
	defaultQuota int64
}

func NewFileService(
	fileRepo *repository.FileRepository,
	folderRepo *repository.FolderRepository,
	store storage.ObjectStore,
	quotaClient client.QuotaClient,
	defaultQuota int64,
) *FileService {
	return &FileService{
		fileRepo:     fileRepo,
		folderRepo:   folderRepo,
		store:        store,
		quotaClient:  quotaClient,
		defaultQuota: defaultQuota,
	}
}

type UploadFileInput struct {
	Name           string
	ParentFolderID *string
	Content        []byte
	ContentType    string
}

// ValidateFileInput proverava naziv datoteke, velicinu i whitelisting ekstenzije
func ValidateFileInput(fileName string, content []byte) (string, string, error) {
	cleaned := strings.TrimSpace(fileName)
	if cleaned == "" || len(cleaned) > 150 || strings.ContainsAny(cleaned, "/\\") || cleaned == "." || cleaned == ".." {
		return "", "", ErrInvalidFileName
	}

	if len(content) == 0 {
		return "", "", ErrEmptyFileContent
	}

	ext := strings.ToLower(filepath.Ext(cleaned))
	if ext == "" || !allowedExtensions[ext] {
		return "", "", fmt.Errorf("%w: ekstenzija '%s' nije na listi dozvoljenih", ErrDisallowedFileExt, ext)
	}

	detectedMime := http.DetectContentType(content)
	return cleaned, detectedMime, nil
}

// CalculateChecksum racuna SHA-256 hes sadrzaja datoteke za proveru integriteta
func CalculateChecksum(content []byte) string {
	sum := sha256.Sum256(content)
	return hex.EncodeToString(sum[:])
}

// UploadFile otprema novu datoteku u izabrani folder ili kreira novu verziju ako datoteka sa istim imenom vec postoji u tom folderu :
// 1. Validira naziv i tip datoteke (whitelisting) i racuna SHA-256 checksum
// 2. Proverava postojanje i vlasnistvo nad roditeljskim folderom
// 3. Postuje kvotu skladista dodeljenu korisniku
// 4. Cuva binarni sadrzaj datoteke i cuva/azurira metapodatke i verziju u MongoDB-u
func (s *FileService) UploadFile(ctx context.Context, ownerID string, input UploadFileInput) (*model.FileMetadata, bool, error) {
	cleanName, detectedMime, err := ValidateFileInput(input.Name, input.Content)
	if err != nil {
		return nil, false, err
	}

	mimeType := strings.TrimSpace(input.ContentType)
	if mimeType == "" || mimeType == "application/octet-stream" {
		mimeType = detectedMime
	}

	// provera roditeljskog foldera (ako nije root)
	var parentID *string
	if input.ParentFolderID != nil && strings.TrimSpace(*input.ParentFolderID) != "" && strings.TrimSpace(*input.ParentFolderID) != "root" {
		cleanedParent := strings.TrimSpace(*input.ParentFolderID)
		parentID = &cleanedParent

		folder, err := s.folderRepo.FindByID(ctx, *parentID)
		if err != nil {
			return nil, false, err
		}
		if folder.OwnerID != ownerID {
			return nil, false, ErrParentNotOwned
		}
	}

	newSize := int64(len(input.Content))
	checksum := CalculateChecksum(input.Content)

	// proveravamo da li u ovom folderu vec postoji datoteka sa istim nazivom
	existing, err := s.fileRepo.FindByNameAndFolder(ctx, ownerID, parentID, cleanName)
	if err != nil {
		return nil, false, err
	}

	// SLUCAJ A: Datoteka vec postoji u tom folderu -> otpremanje NOVE VERZIJE
	if existing != nil {
		updatedFile, err := s.replaceWithNewVersion(ctx, ownerID, existing, input.Content, mimeType, checksum)
		if err != nil {
			return nil, false, err
		}
		return updatedFile, true, nil
	}

	// SLUCAJ B: Nova datoteka (verzija 1)
	// 1. Provera kvote skladista korisnika
	if err := s.checkQuota(ctx, ownerID, newSize); err != nil {
		return nil, false, err
	}

	// 2. Cuvanje sadrzaja datoteke
	contentRef := generateObjectKey(ownerID, cleanName)
	if err := s.store.PutObject(ctx, contentRef, input.Content, mimeType); err != nil {
		return nil, false, fmt.Errorf("greska pri cuvanju sadrzaja datoteke: %w", err)
	}

	// 3. Cuvanje metapodataka u MongoDB
	fileMeta := &model.FileMetadata{
		Name:           cleanName,
		MimeType:       mimeType,
		Size:           newSize,
		OwnerID:        ownerID,
		ParentFolderID: parentID,
		ContentRef:     contentRef,
		Version:        1,
		Checksum:       checksum,
	}

	if err := s.fileRepo.Create(ctx, fileMeta); err != nil {
		_ = s.store.DeleteObject(ctx, contentRef)
		return nil, false, err
	}

	// 4. Sinhrono azuriranje zauzeca kvote kod users-service-a
	s.syncQuotaUsed(ctx, ownerID)

	return fileMeta, false, nil
}

// UploadNewVersion direktno otprema novu verziju postojece datoteke po njenom ID-ju (zahtev 1.5)
func (s *FileService) UploadNewVersion(ctx context.Context, ownerID, fileID string, content []byte, contentType string) (*model.FileMetadata, error) {
	existing, err := s.fileRepo.FindByID(ctx, fileID)
	if err != nil {
		return nil, err
	}

	if existing.OwnerID != ownerID {
		return nil, ErrFileNotOwned
	}

	_, detectedMime, err := ValidateFileInput(existing.Name, content)
	if err != nil {
		return nil, err
	}

	mimeType := strings.TrimSpace(contentType)
	if mimeType == "" || mimeType == "application/octet-stream" {
		mimeType = detectedMime
	}

	checksum := CalculateChecksum(content)
	return s.replaceWithNewVersion(ctx, ownerID, existing, content, mimeType, checksum)
}

func (s *FileService) replaceWithNewVersion(
	ctx context.Context,
	ownerID string,
	existing *model.FileMetadata,
	newContent []byte,
	mimeType string,
	checksum string,
) (*model.FileMetadata, error) {
	newSize := int64(len(newContent))
	deltaBytes := newSize - existing.Size

	// 1. Provera kvote za razliku u velicini (deltaBytes)
	if deltaBytes > 0 {
		if err := s.checkQuota(ctx, ownerID, deltaBytes); err != nil {
			return nil, err
		}
	}

	// 2. Zamena sadrzaja datoteke
	if err := s.store.PutObject(ctx, existing.ContentRef, newContent, mimeType); err != nil {
		return nil, fmt.Errorf("greska pri zameni sadrzaja datoteke: %w", err)
	}

	// 3. Azuriranje metapodataka: verzija + 1 i novo vreme poslednje izmene (updatedAt)
	existing.Size = newSize
	existing.MimeType = mimeType
	existing.Version = existing.Version + 1
	existing.Checksum = checksum

	if err := s.fileRepo.UpdateVersion(ctx, existing); err != nil {
		return nil, err
	}

	// 4. Sinhrono azuriranje zauzeca kvote kod users-service-a
	s.syncQuotaUsed(ctx, ownerID)

	return existing, nil
}

func (s *FileService) checkQuota(ctx context.Context, ownerID string, additionalBytes int64) error {
	usedBytes, err := s.fileRepo.GetTotalUsedBytes(ctx, ownerID)
	if err != nil {
		return err
	}

	quotaLimit := s.defaultQuota
	if s.quotaClient != nil {
		quotaInfo, err := s.quotaClient.GetUserQuota(ctx, ownerID)
		if err != nil {
			return fmt.Errorf("%w: %v", ErrUsersServiceUnavailable, err)
		}
		if quotaInfo.QuotaLimit > 0 {
			quotaLimit = quotaInfo.QuotaLimit
		}
	}

	if usedBytes+additionalBytes > quotaLimit {
		return ErrQuotaExceeded
	}
	return nil
}

func (s *FileService) syncQuotaUsed(ctx context.Context, ownerID string) {
	if s.quotaClient == nil {
		return
	}
	totalUsed, err := s.fileRepo.GetTotalUsedBytes(ctx, ownerID)
	if err != nil {
		log.Printf("[FILES-SERVICE] Upozorenje: neuspesno racunanje zauzeca za korisnika %s: %v", ownerID, err)
		return
	}
	if err := s.quotaClient.UpdateQuotaUsed(ctx, ownerID, totalUsed); err != nil {
		log.Printf("[FILES-SERVICE] Upozorenje: neuspesno sinhronizovanje kvote sa users-service za korisnika %s: %v", ownerID, err)
	}
}

// GetTotalStorageUsage vraca ukupno zauzece skladista za datog korisnika
func (s *FileService) GetTotalStorageUsage(ctx context.Context, ownerID string) (int64, error) {
	return s.fileRepo.GetTotalUsedBytes(ctx, ownerID)
}

// GetFileMetadata vraca metapodatke datoteke uz proveru vlasnistva
func (s *FileService) GetFileMetadata(ctx context.Context, ownerID, fileID string) (*model.FileMetadata, error) {
	file, err := s.fileRepo.FindByID(ctx, fileID)
	if err != nil {
		return nil, err
	}
	if file.OwnerID != ownerID {
		return nil, ErrFileNotOwned
	}
	return file, nil
}

// ValidateFileName proverava naziv datoteke i whitelisting ekstenzije (zahtev 1.6 i 2.18)
func ValidateFileName(fileName string) (string, error) {
	cleaned := strings.TrimSpace(fileName)
	if cleaned == "" || len(cleaned) > 150 || strings.ContainsAny(cleaned, "/\\<>:\"|?*") || cleaned == "." || cleaned == ".." {
		return "", ErrInvalidFileName
	}

	ext := strings.ToLower(filepath.Ext(cleaned))
	if ext == "" || !allowedExtensions[ext] {
		return "", fmt.Errorf("%w: ekstenzija '%s' nije na listi dozvoljenih", ErrDisallowedFileExt, ext)
	}
	return cleaned, nil
}

// RenameFile menja naziv postojece datoteke uz proveru vlasnistva, validaciju imena/ekstenzije i proveru duplikata (zahtev 1.6)
func (s *FileService) RenameFile(ctx context.Context, ownerID, fileID, newName string) (*model.FileMetadata, error) {
	existing, err := s.fileRepo.FindByID(ctx, fileID)
	if err != nil {
		return nil, err
	}
	if existing.OwnerID != ownerID {
		return nil, ErrFileNotOwned
	}

	trimmed := strings.TrimSpace(newName)
	// Ako korisnik pri preimenovanju nije uneo ekstenziju, zadrzavamo originalnu ekstenziju datoteke
	if trimmed != "" && filepath.Ext(trimmed) == "" {
		origExt := filepath.Ext(existing.Name)
		if origExt != "" {
			trimmed = trimmed + origExt
		}
	}

	cleanName, err := ValidateFileName(trimmed)
	if err != nil {
		return nil, err
	}

	if existing.Name == cleanName {
		return existing, nil
	}

	duplicate, err := s.fileRepo.FindByNameAndFolder(ctx, ownerID, existing.ParentFolderID, cleanName)
	if err != nil {
		return nil, err
	}
	if duplicate != nil && duplicate.ID != existing.ID {
		return nil, repository.ErrFileAlreadyExists
	}

	return s.fileRepo.Rename(ctx, fileID, cleanName)
}

// GetFilesInFolder vraca sve datoteke korisnika u izabranom folderu (ili u root-u ako je folderID == nil)
func (s *FileService) GetFilesInFolder(ctx context.Context, ownerID string, folderID *string) ([]model.FileMetadata, error) {
	return s.fileRepo.FindByFolder(ctx, ownerID, folderID)
}

func generateObjectKey(ownerID, fileName string) string {
	randomBytes := make([]byte, 8)
	_, _ = rand.Read(randomBytes)
	ext := strings.ToLower(filepath.Ext(fileName))
	return fmt.Sprintf("%s/%s%s", ownerID, hex.EncodeToString(randomBytes), ext)
}
