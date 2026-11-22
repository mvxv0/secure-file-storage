export interface ApiResponse<T = any> {
  success: boolean;
  message?: string;
  data?: T;
  error?: string;
}

export interface User {
  id: string;
  email: string;
  firstName: string;
  lastName: string;
  role: string;
  quotaLimit: number;
  quotaUsed: number;
}

export interface Folder {
  id: string;
  name: string;
  ownerId: string;
  parentFolderId?: string | null;
  createdAt: string;
  updatedAt: string;
}

export interface FileMetadata {
  id: string;
  name: string;
  mimeType: string;
  size: number;
  ownerId: string;
  parentFolderId?: string | null;
  version: number;
  createdAt: string;
  updatedAt: string;
}

export interface FolderContentResponse {
  currentFolderId: string | null;
  folders: Folder[];
  files?: FileMetadata[];
  quotaUsed?: number;
}

export interface NotificationItem {
  id: string;
  userId: string;
  type: string;
  resourceId: string;
  resourceName: string;
  message: string;
  isRead: boolean;
  createdAt: string;
}

