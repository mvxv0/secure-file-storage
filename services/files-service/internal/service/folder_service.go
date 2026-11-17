package service

import (
	"context"
	"errors"
	"strings"

	"github.com/mvxv0/secure-file-storage/services/files-service/internal/model"
	"github.com/mvxv0/secure-file-storage/services/files-service/internal/repository"
)

var (
	ErrInvalidFolderName = errors.New("naziv foldera mora imati izmedju 1 i 100 karaktera i ne sme sadrzati znakove: /, \\")
	ErrParentNotOwned    = errors.New("nemate pravo pristupa zadatom roditeljskom folderu")
)

type FolderService struct {
	repo *repository.FolderRepository
}

func NewFolderService(repo *repository.FolderRepository) *FolderService {
	return &FolderService{repo: repo}
}

type CreateFolderInput struct {
	Name           string  `json:"name"`
	ParentFolderID *string `json:"parentFolderId,omitempty"` // nil za korenski folder
}

// CreateFolder kreira novi folder u korenskom folderu ili u nekom podfolderu (zahtev 1.4):
// - podrzava hijerarhiju proizvoljne dubine (parent-reference šablon)
// - vrsi proveru i validaciju naziva foldera (sprecavanje nevalidnih karaktera)
// - ako je naveden roditelj, proverava njegovo postojanje i vlasnistvo
// - sprecava duplirana imena foldera na istom nivou
func (s *FolderService) CreateFolder(ctx context.Context, ownerID string, input CreateFolderInput) (*model.Folder, error) {
	name := strings.TrimSpace(input.Name)

	// validacija naziva foldera (zahtev 2.18 input validation)
	if name == "" || len(name) > 100 || strings.ContainsAny(name, "/\\") || name == "." || name == ".." {
		return nil, ErrInvalidFolderName
	}

	var parentID *string
	if input.ParentFolderID != nil && strings.TrimSpace(*input.ParentFolderID) != "" {
		cleaned := strings.TrimSpace(*input.ParentFolderID)
		parentID = &cleaned

		// provera da li roditeljski folder postoji
		parent, err := s.repo.FindByID(ctx, *parentID)
		if err != nil {
			return nil, err
		}

		// provera da li je trenutno ulogovani korisnik vlasnik roditeljskog foldera
		if parent.OwnerID != ownerID {
			return nil, ErrParentNotOwned
		}
	}

	// provera da li folder sa istim nazivom vec postoji u ovom roditeljskom folderu (ili u root-u)
	existing, err := s.repo.FindByNameAndParent(ctx, ownerID, parentID, name)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		return nil, repository.ErrFolderAlreadyExists
	}

	folder := &model.Folder{
		Name:           name,
		OwnerID:        ownerID,
		ParentFolderID: parentID,
	}

	if err := s.repo.Create(ctx, folder); err != nil {
		return nil, err
	}

	return folder, nil
}

// GetFolderContents vraca podfoldere unutar foldera (ili u root-u ako je folderID == nil)
func (s *FolderService) GetFolderContents(ctx context.Context, ownerID string, folderID *string) ([]model.Folder, error) {
	if folderID != nil {
		// provera da li folder postoji i da li pripada korisniku
		currentFolder, err := s.repo.FindByID(ctx, *folderID)
		if err != nil {
			return nil, err
		}
		if currentFolder.OwnerID != ownerID {
			return nil, ErrParentNotOwned
		}
	}

	return s.repo.FindSubfolders(ctx, ownerID, folderID)
}

// GetFolderDetails vraca detalje o jednom folderu
func (s *FolderService) GetFolderDetails(ctx context.Context, ownerID, folderID string) (*model.Folder, error) {
	folder, err := s.repo.FindByID(ctx, folderID)
	if err != nil {
		return nil, err
	}
	if folder.OwnerID != ownerID {
		return nil, ErrParentNotOwned
	}
	return folder, nil
}

