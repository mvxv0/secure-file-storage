import { Injectable } from '@angular/core';
import { HttpClient } from '@angular/common/http';
import { Observable } from 'rxjs';
import { ApiResponse, Folder, FolderContentResponse } from '../models/types';

@Injectable({
  providedIn: 'root'
})
export class FolderService {
  private apiUrl = 'http://localhost:8080/api/folders';

  constructor(private http: HttpClient) {}

  createFolder(name: string, parentFolderId?: string | null): Observable<ApiResponse<Folder>> {
    const body: { name: string; parentFolderId?: string } = { name };
    if (parentFolderId) {
      body.parentFolderId = parentFolderId;
    }
    return this.http.post<ApiResponse<Folder>>(this.apiUrl, body);
  }

  getFolderContents(folderId?: string | null): Observable<ApiResponse<FolderContentResponse>> {
    const url = folderId ? `${this.apiUrl}/${folderId}` : this.apiUrl;
    return this.http.get<ApiResponse<FolderContentResponse>>(url);
  }

  renameFolder(folderId: string, name: string): Observable<ApiResponse<Folder>> {
    return this.http.patch<ApiResponse<Folder>>(`${this.apiUrl}/${folderId}`, { name });
  }
}

