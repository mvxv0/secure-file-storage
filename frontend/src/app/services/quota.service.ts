import { Injectable } from '@angular/core';
import { HttpClient } from '@angular/common/http';
import { Observable } from 'rxjs';
import { ApiResponse, QuotaRequest, User } from '../models/types';

@Injectable({
  providedIn: 'root'
})
export class QuotaService {
  private apiUrl = 'https://localhost:8080/api/users';

  constructor(private http: HttpClient) {}

  getProfile(): Observable<ApiResponse<User>> {
    return this.http.get<ApiResponse<User>>(`${this.apiUrl}/me`);
  }

  submitQuotaRequest(requestedQuotaBytes: number, reason: string): Observable<ApiResponse<QuotaRequest>> {
    return this.http.post<ApiResponse<QuotaRequest>>(`${this.apiUrl}/quota-requests`, {
      requestedQuota: requestedQuotaBytes,
      reason
    });
  }

  getMyQuotaRequests(): Observable<ApiResponse<QuotaRequest[]>> {
    return this.http.get<ApiResponse<QuotaRequest[]>>(`${this.apiUrl}/quota-requests/me`);
  }

  getAllUsers(): Observable<ApiResponse<User[]>> {
    return this.http.get<ApiResponse<User[]>>(this.apiUrl);
  }

  getAllQuotaRequests(): Observable<ApiResponse<QuotaRequest[]>> {
    return this.http.get<ApiResponse<QuotaRequest[]>>(`${this.apiUrl}/quota-requests`);
  }

  approveQuotaRequest(requestId: string): Observable<ApiResponse<QuotaRequest>> {
    return this.http.post<ApiResponse<QuotaRequest>>(`${this.apiUrl}/quota-requests/${requestId}/approve`, {});
  }

  rejectQuotaRequest(requestId: string): Observable<ApiResponse<QuotaRequest>> {
    return this.http.post<ApiResponse<QuotaRequest>>(`${this.apiUrl}/quota-requests/${requestId}/reject`, {});
  }
}

