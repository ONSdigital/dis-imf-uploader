package validation

import (
	"fmt"
	"mime"
	"net/http"
	"path/filepath"
	"strings"
)

// FileValidator validates uploaded files
type FileValidator struct {
	AllowedExtensions []string
	AllowedMimeTypes  []string
	MaxSize           int64
}

// ValidationResult contains validation results
type ValidationResult struct {
	Valid        bool
	Errors       []string
	FileType     string
	DetectedMime string
}

// NewFileValidator creates a new file validator
func NewFileValidator(maxSize int64, allowedExts, allowedMimes []string) *FileValidator {
	return &FileValidator{
		AllowedExtensions: allowedExts,
		AllowedMimeTypes:  allowedMimes,
		MaxSize:           maxSize,
	}
}

// Validate validates a file
func (v *FileValidator) Validate(fileName string, fileData []byte) *ValidationResult {
	result := &ValidationResult{
		Valid:  true,
		Errors: []string{},
	}

	// Check file size
	if int64(len(fileData)) > v.MaxSize {
		result.Valid = false
		result.Errors = append(result.Errors,
			fmt.Sprintf("file size %d exceeds maximum %d bytes", len(fileData), v.MaxSize))
	}

	// Check file extension
	ext := strings.ToLower(filepath.Ext(fileName))
	if !v.isAllowedExtension(ext) {
		result.Valid = false
		result.Errors = append(result.Errors,
			fmt.Sprintf("file extension '%s' not allowed", ext))
	}
	result.FileType = ext

	// Detect MIME type from content
	detectedMime := http.DetectContentType(fileData)
	result.DetectedMime = detectedMime

	// Verify MIME type matches allowed types
	if !v.isAllowedMimeType(detectedMime) {
		result.Valid = false
		result.Errors = append(result.Errors,
			fmt.Sprintf("detected MIME type '%s' not allowed", detectedMime))
	}

	// Check for MIME/extension mismatch
	if !v.mimeMatchesExtension(ext, detectedMime) {
		result.Valid = false
		result.Errors = append(result.Errors,
			fmt.Sprintf("file extension '%s' does not match detected content type '%s'", ext, detectedMime))
	}

	return result
}

func (v *FileValidator) isAllowedExtension(ext string) bool {
	if len(v.AllowedExtensions) == 0 {
		return true // No restrictions
	}

	ext = strings.ToLower(ext)
	for _, allowed := range v.AllowedExtensions {
		if strings.ToLower(allowed) == ext {
			return true
		}
	}
	return false
}

func (v *FileValidator) isAllowedMimeType(mimeType string) bool {
	if len(v.AllowedMimeTypes) == 0 {
		return true // No restrictions
	}

	// Extract base MIME type (without parameters)
	baseMime := strings.Split(mimeType, ";")[0]
	baseMime = strings.TrimSpace(baseMime)

	for _, allowed := range v.AllowedMimeTypes {
		allowedBase := strings.Split(allowed, ";")[0]
		allowedBase = strings.TrimSpace(allowedBase)

		if strings.EqualFold(baseMime, allowedBase) {
			return true
		}

		// Support wildcards like "application/*"
		if strings.HasSuffix(allowedBase, "/*") {
			prefix := strings.TrimSuffix(allowedBase, "/*")
			if strings.HasPrefix(baseMime, prefix+"/") {
				return true
			}
		}
	}
	return false
}

func (v *FileValidator) mimeMatchesExtension(ext, mimeType string) bool {
	// Get expected MIME types for this extension
	expectedMimes := mime.TypeByExtension(ext)
	if expectedMimes == "" {
		// Unknown extension, can't verify - allow it
		return true
	}

	baseMime := strings.Split(mimeType, ";")[0]
	baseMime = strings.TrimSpace(baseMime)

	expectedBase := strings.Split(expectedMimes, ";")[0]
	expectedBase = strings.TrimSpace(expectedBase)

	// Allow some common mismatches
	// For example, PDF files are sometimes detected as application/octet-stream
	if ext == ".pdf" && (baseMime == "application/pdf" || baseMime == "application/octet-stream") {
		return true
	}

	// Excel files
	if (ext == ".xlsx" || ext == ".xls") &&
		(strings.HasPrefix(baseMime, "application/vnd.") || baseMime == "application/octet-stream") {
		return true
	}

	// Word files
	if (ext == ".docx" || ext == ".doc") &&
		(strings.HasPrefix(baseMime, "application/vnd.") || baseMime == "application/octet-stream") {
		return true
	}

	return strings.EqualFold(baseMime, expectedBase) || baseMime == "application/octet-stream"
}

// GetErrorMessage returns a formatted error message
func (r *ValidationResult) GetErrorMessage() string {
	if r.Valid {
		return ""
	}
	return strings.Join(r.Errors, "; ")
}
