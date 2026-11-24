import { Injectable } from '@angular/core';
import { HttpClient } from '@angular/common/http';
import { Observable } from 'rxjs';
import { ApiResponse, NotificationItem } from '../models/types';

@Injectable({
  providedIn: 'root'
})
export class NotificationService {
  private apiUrl = 'https://localhost:8080/api/notifications';

  constructor(private http: HttpClient) {}

  getNotifications(): Observable<ApiResponse<NotificationItem[]>> {
    return this.http.get<ApiResponse<NotificationItem[]>>(this.apiUrl);
  }

  createNotification(payload: {
    type: string;
    resourceName: string;
    message: string;
  }): Observable<ApiResponse<NotificationItem>> {
    return this.http.post<ApiResponse<NotificationItem>>(this.apiUrl, payload);
  }

  seedNotifications(): Observable<ApiResponse<NotificationItem[]>> {
    return this.http.post<ApiResponse<NotificationItem[]>>(`${this.apiUrl}/seed`, {});
  }

  markAllAsRead(): Observable<ApiResponse<void>> {
    return this.http.post<ApiResponse<void>>(`${this.apiUrl}/read`, {});
  }
}
