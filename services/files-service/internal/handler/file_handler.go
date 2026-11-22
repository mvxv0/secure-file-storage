package handler

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/mvxv0/secure-file-storage/common/auth"
	"github.com/mvxv0/secure-file-storage/common/httputil"
	"github.com/mvxv0/secure-file-storage/services/files-service/internal/repository"
	"github.com/mvxv0/secure-file-storage/services/files-service/internal/service"
)

const maxUploadSize = 50 << 20 // 50 MB po pojedinacnom zahtevu

type FileHandler struct {
	fileService *service.FileService
}

func NewFileHandler(fileService *service.FileService) *FileHandler {
	return &FileHandler{fileService: fileService}
}

type jsonUploadRequest struct {
	Name           string  `json:"name"`
	ParentFolderID *string `json:"parentFolderId,omitempty"`
	Content        string  `json:"content"`
	MimeType       string  `json:"mimeType,omitempty"`
}

// UploadFile obradjuje otpremanje datoteke u izabrani folder ili otpremanje nove verzije
// Podrzava i multipart/form-data (standardni file upload u Postman-u) i application/json
func (h *FileHandler) UploadFile(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		httputil.WriteError(w, http.StatusMethodNotAllowed, "Dozvoljen je samo POST metod")
		return
	}

	claims, ok := auth.GetClaims(r)
	if !ok || claims.UserID == "" {
		httputil.WriteError(w, http.StatusUnauthorized, "Neautorizovan pristup")
		return
	}

	var input service.UploadFileInput
	contentTypeHeader := r.Header.Get("Content-Type")

	if strings.HasPrefix(contentTypeHeader, "multipart/form-data") {
		if err := r.ParseMultipartForm(maxUploadSize); err != nil {
			httputil.WriteError(w, http.StatusBadRequest, "Neuspesno citanje multipart forme (maksimalna velicina je 50MB)")
			return
		}

		file, fileHeader, err := r.FormFile("file")
		if err != nil {
			httputil.WriteError(w, http.StatusBadRequest, "Polje 'file' u form-data je obavezno")
			return
		}
		defer file.Close()

		content, err := io.ReadAll(file)
		if err != nil {
			httputil.WriteError(w, http.StatusInternalServerError, "Greska pri citanju sadrzaja datoteke")
			return
		}

		customName := strings.TrimSpace(r.FormValue("name"))
		if customName == "" {
			customName = fileHeader.Filename
		}

		var parentID *string
		if rawParent := strings.TrimSpace(r.FormValue("parentFolderId")); rawParent != "" {
			parentID = &rawParent
		}

		input = service.UploadFileInput{
			Name:           customName,
			ParentFolderID: parentID,
			Content:        content,
			ContentType:    fileHeader.Header.Get("Content-Type"),
		}
	} else {
		var jsonReq jsonUploadRequest
		if err := json.NewDecoder(r.Body).Decode(&jsonReq); err != nil {
			httputil.WriteError(w, http.StatusBadRequest, "Posaljite multipart/form-data (polje 'file') ili validan JSON")
			return
		}
		input = service.UploadFileInput{
			Name:           jsonReq.Name,
			ParentFolderID: jsonReq.ParentFolderID,
			Content:        []byte(jsonReq.Content),
			ContentType:    jsonReq.MimeType,
		}
	}

	fileMeta, isNewVersion, err := h.fileService.UploadFile(r.Context(), claims.UserID, input)
	if err != nil {
		h.writeFileError(w, err)
		return
	}

	if isNewVersion {
		httputil.WriteSuccess(w, http.StatusOK, "Nova verzija datoteke je uspesno otpremljena", fileMeta)
		return
	}

	httputil.WriteSuccess(w, http.StatusCreated, "Datoteka je uspesno otpremljena", fileMeta)
}

// HandleFileByID obradjuje rute oblika:
// - GET /api/files/{id} -> dobavljanje metapodataka datoteke
// - PUT /api/files/{id} ili PUT/POST /api/files/{id}/version -> otpremanje nove verzije postojece datoteke
func (h *FileHandler) HandleFileByID(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.GetClaims(r)
	if !ok || claims.UserID == "" {
		httputil.WriteError(w, http.StatusUnauthorized, "Neautorizovan pristup")
		return
	}

	trimmed := strings.TrimPrefix(r.URL.Path, "/api/files/")
	parts := strings.Split(strings.Trim(trimmed, "/"), "/")
	if len(parts) == 0 || parts[0] == "" {
		httputil.WriteError(w, http.StatusBadRequest, "ID datoteke je obavezan")
		return
	}

	fileID := parts[0]

	// POST /api/files/upload (alternativna putanja za upload)
	if fileID == "upload" && r.Method == http.MethodPost {
		h.UploadFile(w, r)
		return
	}

	// GET /api/files/{id}
	if len(parts) == 1 && r.Method == http.MethodGet {
		fileMeta, err := h.fileService.GetFileMetadata(r.Context(), claims.UserID, fileID)
		if err != nil {
			h.writeFileError(w, err)
			return
		}
		httputil.WriteSuccess(w, http.StatusOK, "Metapodaci datoteke uspesno dobavljeni", fileMeta)
		return
	}

	// PUT /api/files/{id} ili PUT/POST /api/files/{id}/version -> otpremanje nove verzije
	isVersionRoute := (len(parts) == 1 && r.Method == http.MethodPut) ||
		(len(parts) == 2 && parts[1] == "version" && (r.Method == http.MethodPut || r.Method == http.MethodPost))

	if isVersionRoute {
		content, contentType, err := extractUploadedContent(r)
		if err != nil {
			httputil.WriteError(w, http.StatusBadRequest, err.Error())
			return
		}

		updatedMeta, err := h.fileService.UploadNewVersion(r.Context(), claims.UserID, fileID, content, contentType)
		if err != nil {
			h.writeFileError(w, err)
			return
		}

		httputil.WriteSuccess(w, http.StatusOK, "Nova verzija datoteke je uspesno otpremljena", updatedMeta)
		return
	}

	httputil.WriteError(w, http.StatusMethodNotAllowed, "Metod ili putanja nisu podrzani")
}

// ListFiles vraca datoteke u izabranom folderu (?folderId=...) ili u root folderu
func (h *FileHandler) ListFiles(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		httputil.WriteError(w, http.StatusMethodNotAllowed, "Dozvoljen je samo GET metod")
		return
	}

	claims, ok := auth.GetClaims(r)
	if !ok || claims.UserID == "" {
		httputil.WriteError(w, http.StatusUnauthorized, "Neautorizovan pristup")
		return
	}

	var folderID *string
	if q := strings.TrimSpace(r.URL.Query().Get("folderId")); q != "" && q != "root" {
		folderID = &q
	}

	files, err := h.fileService.GetFilesInFolder(r.Context(), claims.UserID, folderID)
	if err != nil {
		h.writeFileError(w, err)
		return
	}

	httputil.WriteSuccess(w, http.StatusOK, "Datoteke uspesno dobavljene", files)
}

func extractUploadedContent(r *http.Request) ([]byte, string, error) {
	if strings.HasPrefix(r.Header.Get("Content-Type"), "multipart/form-data") {
		if err := r.ParseMultipartForm(maxUploadSize); err != nil {
			return nil, "", errors.New("neuspesno citanje multipart forme")
		}
		file, fileHeader, err := r.FormFile("file")
		if err != nil {
			return nil, "", errors.New("polje 'file' u form-data je obavezno")
		}
		defer file.Close()

		content, err := io.ReadAll(file)
		if err != nil {
			return nil, "", errors.New("greska pri citanju sadrzaja datoteke")
		}
		return content, fileHeader.Header.Get("Content-Type"), nil
	}

	var jsonReq jsonUploadRequest
	if err := json.NewDecoder(r.Body).Decode(&jsonReq); err != nil {
		return nil, "", errors.New("posaljite multipart/form-data (polje 'file') ili validan JSON")
	}
	return []byte(jsonReq.Content), jsonReq.MimeType, nil
}

func (h *FileHandler) writeFileError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, service.ErrInvalidFileName),
		errors.Is(err, service.ErrEmptyFileContent),
		errors.Is(err, service.ErrDisallowedFileExt):
		httputil.WriteError(w, http.StatusBadRequest, err.Error())
	case errors.Is(err, repository.ErrFolderNotFound),
		errors.Is(err, repository.ErrFileNotFound):
		httputil.WriteError(w, http.StatusNotFound, err.Error())
	case errors.Is(err, service.ErrParentNotOwned),
		errors.Is(err, service.ErrFileNotOwned):
		httputil.WriteError(w, http.StatusForbidden, err.Error())
	case errors.Is(err, service.ErrQuotaExceeded):
		httputil.WriteError(w, http.StatusRequestEntityTooLarge, err.Error())
	case errors.Is(err, service.ErrUsersServiceUnavailable):
		httputil.WriteError(w, http.StatusServiceUnavailable, err.Error())
	case errors.Is(err, repository.ErrFileAlreadyExists):
		httputil.WriteError(w, http.StatusConflict, err.Error())
	default:
		httputil.WriteError(w, http.StatusInternalServerError, "Greska pri obradi datoteke: "+err.Error())
	}
}
