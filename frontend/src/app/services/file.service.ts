import { Injectable } from '@angular/core';
import { HttpClient } from '@angular/common/http';
import { Observable } from 'rxjs';
import { ApiResponse, FileMetadata } from '../models/types';

@Injectable({
  providedIn: 'root'
})
export class FileService {
  private apiUrl = 'http://localhost:8080/api/files';

  constructor(private http: HttpClient) {}

  uploadFile(file: File, parentFolderId?: string | null): Observable<ApiResponse<FileMetadata>> {
    const formData = new FormData();
    formData.append('file', file);
    if (parentFolderId) {
      formData.append('parentFolderId', parentFolderId);
    }
    return this.http.post<ApiResponse<FileMetadata>>(this.apiUrl, formData);
  }

  downloadFile(fileId: string): Observable<Blob> {
    return this.http.get(`${this.apiUrl}/${fileId}/download`, { responseType: 'blob' });
  }

  renameFile(fileId: string, name: string): Observable<ApiResponse<FileMetadata>> {
    return this.http.patch<ApiResponse<FileMetadata>>(`${this.apiUrl}/${fileId}`, { name });
  }
}

