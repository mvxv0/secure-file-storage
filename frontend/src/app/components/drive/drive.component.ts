import { Component, OnInit, signal } from '@angular/core';
import { CommonModule } from '@angular/common';
import { FormsModule } from '@angular/forms';
import { AuthService } from '../../services/auth.service';
import { FolderService } from '../../services/folder.service';
import { FileService } from '../../services/file.service';
import { Folder, FileMetadata, User } from '../../models/types';

interface BreadcrumbItem {
  id: string | null;
  name: string;
}

@Component({
  selector: 'app-drive',
  standalone: true,
  imports: [CommonModule, FormsModule],
  templateUrl: './drive.component.html',
  styleUrl: './drive.component.css'
})
export class DriveComponent implements OnInit {
  user = signal<User | null>(null);

  // Trenutna lokacija
  currentFolderId = signal<string | null>(null);
  breadcrumbs = signal<BreadcrumbItem[]>([
    { id: null, name: 'Moj Disk (Koren)' }
  ]);

  // Sadrzaj foldera
  folders = signal<Folder[]>([]);
  files = signal<FileMetadata[]>([]);

  // Novi folder modal / prompt
  isCreateFolderOpen = signal<boolean>(false);
  newFolderName = '';

  // Poruke i status
  errorMessage = signal<string | null>(null);
  successMessage = signal<string | null>(null);
  isLoading = signal<boolean>(false);

  constructor(
    private authService: AuthService,
    private folderService: FolderService,
    private fileService: FileService
  ) {}

  ngOnInit(): void {
    this.user.set(this.authService.currentUser());
    this.loadCurrentFolder();
  }

  loadCurrentFolder(): void {
    this.isLoading.set(true);
    this.errorMessage.set(null);

    this.folderService.getFolderContents(this.currentFolderId()).subscribe({
      next: (res) => {
        this.isLoading.set(false);
        if (res.success && res.data) {
          this.folders.set(res.data.folders || []);
          this.files.set(res.data.files || []);
          if (res.data.quotaUsed !== undefined) {
            this.user.update(u => u ? { ...u, quotaUsed: res.data!.quotaUsed! } : null);
          }
        }
      },
      error: (err) => {
        this.isLoading.set(false);
        this.errorMessage.set(err.error?.error || 'Greska pri ucitavanju sadrzaja foldera.');
      }
    });
  }

  // Navigacija u podfolder
  openFolder(folder: Folder): void {
    this.currentFolderId.set(folder.id);
    this.breadcrumbs.update(crumbs => [...crumbs, { id: folder.id, name: folder.name }]);
    this.loadCurrentFolder();
  }

  // Navigacija preko Breadcrumb-a nazad
  goToBreadcrumb(index: number): void {
    const crumbs = this.breadcrumbs();
    const target = crumbs[index];
    this.currentFolderId.set(target.id);
    this.breadcrumbs.set(crumbs.slice(0, index + 1));
    this.loadCurrentFolder();
  }

  // --- KREIRANJE FOLDERA ---
  openCreateFolderModal(): void {
    this.newFolderName = '';
    this.isCreateFolderOpen.set(true);
  }

  closeCreateFolderModal(): void {
    this.isCreateFolderOpen.set(false);
    this.newFolderName = '';
  }

  createFolder(): void {
    if (!this.newFolderName.trim()) return;

    this.folderService.createFolder(this.newFolderName.trim(), this.currentFolderId()).subscribe({
      next: (res) => {
        if (res.success) {
          this.closeCreateFolderModal();
          this.successMessage.set(`Folder "${this.newFolderName}" je uspesno kreiran!`);
          setTimeout(() => this.successMessage.set(null), 3000);
          this.loadCurrentFolder();
        }
      },
      error: (err) => {
        this.errorMessage.set(err.error?.error || 'Greska pri kreiranju foldera.');
        setTimeout(() => this.errorMessage.set(null), 4000);
      }
    });
  }

  // --- UPLOAD FAJLA ---
  onFileSelected(event: Event): void {
    const input = event.target as HTMLInputElement;
    if (!input.files || input.files.length === 0) return;

    const file = input.files[0];
    this.isLoading.set(true);
    this.errorMessage.set(null);

    this.fileService.uploadFile(file, this.currentFolderId()).subscribe({
      next: (res) => {
        this.isLoading.set(false);
        input.value = '';
        if (res.success) {
          this.successMessage.set(`Datoteka "${file.name}" uspesno otpremljena!`);
          setTimeout(() => this.successMessage.set(null), 3000);
          this.loadCurrentFolder();
        }
      },
      error: (err) => {
        this.isLoading.set(false);
        input.value = '';
        this.errorMessage.set(err.error?.error || 'Greska pri otpremanju datoteke.');
        setTimeout(() => this.errorMessage.set(null), 5000);
      }
    });
  }

  // --- PREUZIMANJE FAJLA ---
  downloadFile(file: FileMetadata): void {
    this.fileService.downloadFile(file.id).subscribe({
      next: (blob) => {
        const url = window.URL.createObjectURL(blob);
        const a = document.createElement('a');
        a.href = url;
        a.download = file.name;
        a.click();
        window.URL.revokeObjectURL(url);
      },
      error: (err) => {
        this.errorMessage.set('Greska pri preuzimanju datoteke.');
        setTimeout(() => this.errorMessage.set(null), 4000);
      }
    });
  }

  formatSize(bytes: number): string {
    if (bytes === 0) return '0 B';
    const k = 1024;
    const sizes = ['B', 'KB', 'MB', 'GB'];
    const i = Math.floor(Math.log(bytes) / Math.log(k));
    return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + ' ' + sizes[i];
  }

  formatQuota(user: User | null): string {
    if (!user) return '';
    return `${this.formatSize(user.quotaUsed)} / ${this.formatSize(user.quotaLimit)}`;
  }

  logout(): void {
    this.authService.logout();
  }
}

