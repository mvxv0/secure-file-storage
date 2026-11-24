import { Injectable, signal } from '@angular/core';
import { HttpClient } from '@angular/common/http';
import { Observable, tap } from 'rxjs';
import { ApiResponse, User } from '../models/types';
import { Router } from '@angular/router';

@Injectable({
  providedIn: 'root'
})
export class AuthService {
  private apiUrl = 'https://localhost:8080/api/auth';
  private usersUrl = 'https://localhost:8080/api/users';
  
  currentUser = signal<User | null>(this.getStoredUser());

  constructor(private http: HttpClient, private router: Router) {}

  register(data: {
    firstName: string;
    lastName: string;
    email: string;
    password: string;
    confirmPassword: string;
  }): Observable<ApiResponse> {
    return this.http.post<ApiResponse>(`${this.apiUrl}/register`, data);
  }

  verifyEmail(email: string, code: string): Observable<ApiResponse> {
    return this.http.post<ApiResponse>(`${this.apiUrl}/verify-email`, { email, code });
  }

  login(email: string, password: string): Observable<ApiResponse<{ step: string; email: string }>> {
    return this.http.post<ApiResponse<{ step: string; email: string }>>(`${this.apiUrl}/login`, { email, password });
  }

  verifyOtp(email: string, code: string): Observable<ApiResponse<{ token: string; user: User }>> {
    return this.http.post<ApiResponse<{ token: string; user: User }>>(`${this.apiUrl}/verify-otp`, { email, code }).pipe(
      tap(res => {
        if (res.success && res.data) {
          localStorage.setItem('auth_token', res.data.token);
          localStorage.setItem('auth_user', JSON.stringify(res.data.user));
          this.currentUser.set(res.data.user);
        }
      })
    );
  }

  forgotPassword(email: string): Observable<ApiResponse> {
    return this.http.post<ApiResponse>(`${this.apiUrl}/forgot-password`, { email });
  }

  resetPassword(data: {
    email: string;
    token: string;
    newPassword: string;
    confirmPassword: string;
  }): Observable<ApiResponse> {
    return this.http.post<ApiResponse>(`${this.apiUrl}/reset-password`, data);
  }

  requestMagicLink(email: string): Observable<ApiResponse> {
    return this.http.post<ApiResponse>(`${this.apiUrl}/magic-link`, { email });
  }

  verifyMagicLink(email: string, token: string): Observable<ApiResponse<{ token: string; user: User }>> {
    return this.http.post<ApiResponse<{ token: string; user: User }>>(`${this.apiUrl}/magic-link/verify`, { email, token }).pipe(
      tap(res => {
        if (res.success && res.data) {
          localStorage.setItem('auth_token', res.data.token);
          localStorage.setItem('auth_user', JSON.stringify(res.data.user));
          this.currentUser.set(res.data.user);
        }
      })
    );
  }

  getProfile(): Observable<ApiResponse<User>> {
    return this.http.get<ApiResponse<User>>(`${this.usersUrl}/me`);
  }

  requestAccountUpdate(): Observable<ApiResponse> {
    return this.http.post<ApiResponse>(`${this.usersUrl}/me/request-update`, {});
  }

  updateAccount(data: {
    code: string;
    firstName: string;
    lastName: string;
    newPassword?: string;
    confirmPassword?: string;
  }): Observable<ApiResponse<User>> {
    return this.http.put<ApiResponse<User>>(`${this.usersUrl}/me`, data).pipe(
      tap(res => {
        if (res.success && res.data) {
          localStorage.setItem('auth_user', JSON.stringify(res.data));
          this.currentUser.set(res.data);
        }
      })
    );
  }

  logout(): void {
    const token = this.getToken();
    if (token) {
      this.http.post(`${this.apiUrl}/logout`, {}).subscribe({
        complete: () => this.clearSession(),
        error: () => this.clearSession()
      });
    } else {
      this.clearSession();
    }
  }

  private clearSession(): void {
    localStorage.removeItem('auth_token');
    localStorage.removeItem('auth_user');
    this.currentUser.set(null);
    this.router.navigate(['/auth']);
  }

  getToken(): string | null {
    return localStorage.getItem('auth_token');
  }

  isLoggedIn(): boolean {
    return !!this.getToken();
  }

  private getStoredUser(): User | null {
    const userStr = localStorage.getItem('auth_user');
    if (!userStr) return null;
    try {
      return JSON.parse(userStr);
    } catch {
      return null;
    }
  }
}

