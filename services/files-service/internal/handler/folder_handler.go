package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/mvxv0/secure-file-storage/common/auth"
	"github.com/mvxv0/secure-file-storage/common/httputil"
	"github.com/mvxv0/secure-file-storage/services/files-service/internal/model"
	"github.com/mvxv0/secure-file-storage/services/files-service/internal/repository"
	"github.com/mvxv0/secure-file-storage/services/files-service/internal/service"
)

type FolderHandler struct {
	folderService *service.FolderService
	fileService   *service.FileService
}

func NewFolderHandler(folderService *service.FolderService, fileService *service.FileService) *FolderHandler {
	return &FolderHandler{
		folderService: folderService,
		fileService:   fileService,
	}
}

// CreateFolder kreira folder u root-u ili kao podfolder (zahtev 1.4)
func (h *FolderHandler) CreateFolder(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		httputil.WriteError(w, http.StatusMethodNotAllowed, "Dozvoljen je samo POST metod")
		return
	}

	claims, ok := auth.GetClaims(r)
	if !ok || claims.UserID == "" {
		httputil.WriteError(w, http.StatusUnauthorized, "Neautorizovan pristup")
		return
	}

	var input service.CreateFolderInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		httputil.WriteError(w, http.StatusBadRequest, "Nevalidan JSON format tela zahteva")
		return
	}

	folder, err := h.folderService.CreateFolder(r.Context(), claims.UserID, input)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrInvalidFolderName):
			httputil.WriteError(w, http.StatusBadRequest, err.Error())
		case errors.Is(err, repository.ErrFolderNotFound):
			httputil.WriteError(w, http.StatusNotFound, "Navedeni roditeljski folder ne postoji")
		case errors.Is(err, service.ErrParentNotOwned):
			httputil.WriteError(w, http.StatusForbidden, err.Error())
		case errors.Is(err, repository.ErrFolderAlreadyExists):
			httputil.WriteError(w, http.StatusConflict, err.Error())
		default:
			httputil.WriteError(w, http.StatusInternalServerError, "Greska pri kreiranju foldera: "+err.Error())
		}
		return
	}

	httputil.WriteSuccess(w, http.StatusCreated, "Folder je uspesno kreiran", folder)
}

// GetFolderContents vraca podfoldere i datoteke unutar korenskog foldera ili odredjenog podfoldera
func (h *FolderHandler) GetFolderContents(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		httputil.WriteError(w, http.StatusMethodNotAllowed, "Dozvoljen je samo GET metod")
		return
	}

	claims, ok := auth.GetClaims(r)
	if !ok || claims.UserID == "" {
		httputil.WriteError(w, http.StatusUnauthorized, "Neautorizovan pristup")
		return
	}

	// putanja moze biti /api/folders ili /api/folders/{id}
	path := strings.TrimPrefix(r.URL.Path, "/api/folders")
	path = strings.TrimPrefix(path, "/")

	var folderID *string
	if path != "" && path != "root" {
		folderID = &path
	}

	folders, err := h.folderService.GetFolderContents(r.Context(), claims.UserID, folderID)
	if err != nil {
		switch {
		case errors.Is(err, repository.ErrFolderNotFound):
			httputil.WriteError(w, http.StatusNotFound, "Folder nije pronadjen")
		case errors.Is(err, service.ErrParentNotOwned):
			httputil.WriteError(w, http.StatusForbidden, err.Error())
		default:
			httputil.WriteError(w, http.StatusInternalServerError, "Greska pri dobavljanju sadrzaja foldera: "+err.Error())
		}
		return
	}

	files := make([]model.FileMetadata, 0)
	var quotaUsed int64 = 0
	if h.fileService != nil {
		if fList, fErr := h.fileService.GetFilesInFolder(r.Context(), claims.UserID, folderID); fErr == nil {
			files = fList
		}
		if used, uErr := h.fileService.GetTotalStorageUsage(r.Context(), claims.UserID); uErr == nil {
			quotaUsed = used
		}
	}

	httputil.WriteSuccess(w, http.StatusOK, "Sadrzaj foldera uspesno dobavljen", map[string]interface{}{
		"currentFolderId": folderID,
		"folders":         folders,
		"files":           files,
		"quotaUsed":       quotaUsed,
	})
}
