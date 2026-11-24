import { Component, OnInit, signal } from '@angular/core';
import { CommonModule } from '@angular/common';
import { FormsModule } from '@angular/forms';
import { AuthService } from '../../services/auth.service';
import { FolderService } from '../../services/folder.service';
import { FileService } from '../../services/file.service';
import { NotificationService } from '../../services/notification.service';
import { Folder, FileMetadata, User, NotificationItem } from '../../models/types';

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

  // Izmena podataka korisnickog naloga (US 02.04)
  isProfileModalOpen = signal<boolean>(false);
  profileFirstName = '';
  profileLastName = '';
  profileNewPassword = '';
  profileConfirmPassword = '';
  profileVerificationCode = '';
  profileUpdateCodeSent = signal<boolean>(false);
  isProfileLoading = signal<boolean>(false);
  // Preimenovanje foldera ili datoteke (zahtev 1.6)
  isRenameModalOpen = signal<boolean>(false);
  renameTargetType = signal<'folder' | 'file'>('folder');
  renameTargetId = signal<string>('');
  renameNewName = '';

  // Notifikacije 
  notifications = signal<NotificationItem[]>([]);
  isNotificationsOpen = signal<boolean>(false);
  isNotificationsLoading = signal<boolean>(false);

  // Poruke i status
  errorMessage = signal<string | null>(null);
  successMessage = signal<string | null>(null);
  isLoading = signal<boolean>(false);

  constructor(
    private authService: AuthService,
    private folderService: FolderService,
    private fileService: FileService,
    private notificationService: NotificationService
  ) { }

  ngOnInit(): void {
    this.user.set(this.authService.currentUser());
    this.loadCurrentFolder();
    this.loadNotifications();
  }

  loadNotifications(markReadAfterLoad = false): void {
    this.isNotificationsLoading.set(true);
    this.notificationService.getNotifications().subscribe({
      next: (res) => {
        this.isNotificationsLoading.set(false);
        if (res.success && res.data) {
          this.notifications.set(res.data);
          if (markReadAfterLoad && this.unreadNotificationsCount() > 0) {
            this.markNotificationsAsRead();
          }
        }
      },
      error: () => {
        this.isNotificationsLoading.set(false);
      }
    });
  }

  markNotificationsAsRead(): void {
    this.notificationService.markAllAsRead().subscribe({
      next: () => {
        this.notifications.update(list => list.map(n => ({ ...n, isRead: true })));
      }
    });
  }

  toggleNotificationsModal(): void {
    const nextState = !this.isNotificationsOpen();
    this.isNotificationsOpen.set(nextState);
    if (nextState) {
      this.loadNotifications(true);
    }
  }

  closeNotificationsModal(): void {
    this.isNotificationsOpen.set(false);
  }

  unreadNotificationsCount(): number {
    return this.notifications().filter(n => !n.isRead).length;
  }

  getNotificationIcon(type: string): string {
    switch (type) {
      case 'FOLDER_SHARED':
        return '📁';
      case 'FILE_ADDED_TO_SHARED_FOLDER':
        return '➕';
      case 'NEW_VERSION':
        return '🔄';
      case 'FILE_SHARED':
      default:
        return '📄';
    }
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

  // Dozvoljene ekstenzije na klijentskoj strani ([US 09.02] Validacija ulaznih podataka)
  private readonly allowedExtensions = [
    '.txt', '.md', '.csv', '.json', '.xml', '.pdf',
    '.png', '.jpg', '.jpeg', '.gif', '.webp', '.svg',
    '.doc', '.docx', '.xls', '.xlsx', '.ppt', '.pptx', '.zip'
  ];

  validateItemNameClient(name: string, maxLen = 100): string | null {
    const trimmed = name.trim();
    if (!trimmed) return 'Naziv ne moze biti prazan.';
    if (trimmed.length > maxLen) return `Naziv ne sme biti duzi od ${maxLen} karaktera.`;
    if (/[\/\\<>:"|?*$]/.test(trimmed) || trimmed === '.' || trimmed === '..') {
      return 'Naziv sadrzi nedozvoljene specijalne karaktere.';
    }
    return null;
  }

  createFolder(): void {
    const validationErr = this.validateItemNameClient(this.newFolderName, 100);
    if (validationErr) {
      this.errorMessage.set(validationErr);
      setTimeout(() => this.errorMessage.set(null), 4000);
      return;
    }

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

  // --- PREIMENOVANJE FOLDERA I DATOTEKA (zahtev 1.6) ---
  openRenameFolderModal(folder: Folder, event?: Event): void {
    if (event) event.stopPropagation();
    this.renameTargetType.set('folder');
    this.renameTargetId.set(folder.id);
    this.renameNewName = folder.name;
    this.isRenameModalOpen.set(true);
  }

  openRenameFileModal(file: FileMetadata, event?: Event): void {
    if (event) event.stopPropagation();
    this.renameTargetType.set('file');
    this.renameTargetId.set(file.id);
    this.renameNewName = file.name;
    this.isRenameModalOpen.set(true);
  }

  closeRenameModal(): void {
    this.isRenameModalOpen.set(false);
    this.renameTargetId.set('');
    this.renameNewName = '';
  }

  submitRename(): void {
    const trimmed = this.renameNewName.trim();
    const id = this.renameTargetId();
    if (!trimmed || !id) return;

    if (this.renameTargetType() === 'folder') {
      this.folderService.renameFolder(id, trimmed).subscribe({
        next: (res) => {
          if (res.success) {
            this.closeRenameModal();
            this.successMessage.set('Naziv foldera je uspesno izmenjen!');
            setTimeout(() => this.successMessage.set(null), 3000);
            this.loadCurrentFolder();
          }
        },
        error: (err) => {
          this.errorMessage.set(err.error?.error || 'Greska pri preimenovanju foldera.');
          setTimeout(() => this.errorMessage.set(null), 4000);
        }
      });
    } else {
      this.fileService.renameFile(id, trimmed).subscribe({
        next: (res) => {
          if (res.success) {
            this.closeRenameModal();
            this.successMessage.set('Naziv datoteke je uspesno izmenjen!');
            setTimeout(() => this.successMessage.set(null), 3000);
            this.loadCurrentFolder();
          }
        },
        error: (err) => {
          this.errorMessage.set(err.error?.error || 'Greska pri preimenovanju datoteke.');
          setTimeout(() => this.errorMessage.set(null), 4000);
        }
      });
    }
  }

  // --- UPLOAD FAJLA ---
  onFileSelected(event: Event): void {
    const input = event.target as HTMLInputElement;
    if (!input.files || input.files.length === 0) return;

    const file = input.files[0];

    // [US 09.02] Klijentska validacija naziva, velicine i dozvoljenog tipa datoteke (whitelisting)
    const nameErr = this.validateItemNameClient(file.name, 150);
    if (nameErr) {
      input.value = '';
      this.errorMessage.set(nameErr);
      setTimeout(() => this.errorMessage.set(null), 4000);
      return;
    }
    if (file.size === 0) {
      input.value = '';
      this.errorMessage.set('Datoteka ne moze biti prazna (0 B).');
      setTimeout(() => this.errorMessage.set(null), 4000);
      return;
    }
    const dotIdx = file.name.lastIndexOf('.');
    const ext = dotIdx >= 0 ? file.name.substring(dotIdx).toLowerCase() : '';
    if (!ext || !this.allowedExtensions.includes(ext)) {
      input.value = '';
      this.errorMessage.set(`Tip datoteke "${ext || 'bez ekstenzije'}" nije dozvoljen (whitelisting zastita).`);
      setTimeout(() => this.errorMessage.set(null), 5000);
      return;
    }

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

  // --- IZMENA KORISNICKOG NALOGA (US 02.04) ---
  openProfileModal(): void {
    const u = this.user();
    this.profileFirstName = u?.firstName || '';
    this.profileLastName = u?.lastName || '';
    this.profileNewPassword = '';
    this.profileConfirmPassword = '';
    this.profileVerificationCode = '';
    this.profileUpdateCodeSent.set(false);
    this.isProfileModalOpen.set(true);
  }

  closeProfileModal(): void {
    this.isProfileModalOpen.set(false);
  }

  requestProfileUpdateCode(): void {
    this.isProfileLoading.set(true);
    this.errorMessage.set(null);

    this.authService.requestAccountUpdate().subscribe({
      next: (res) => {
        this.isProfileLoading.set(false);
        this.profileUpdateCodeSent.set(true);
        this.successMessage.set(res.message || 'Verifikacioni kod je poslat na vas email (proverite MailHog na :8025).');
        setTimeout(() => this.successMessage.set(null), 4000);
      },
      error: (err) => {
        this.isProfileLoading.set(false);
        this.errorMessage.set(err.error?.error || 'Greska pri slanju verifikacionog koda.');
      }
    });
  }

  submitProfileUpdate(): void {
    if (!this.profileVerificationCode.trim()) return;
    this.isProfileLoading.set(true);
    this.errorMessage.set(null);

    this.authService.updateAccount({
      code: this.profileVerificationCode.trim(),
      firstName: this.profileFirstName.trim(),
      lastName: this.profileLastName.trim(),
      newPassword: this.profileNewPassword || undefined,
      confirmPassword: this.profileConfirmPassword || undefined
    }).subscribe({
      next: (res) => {
        this.isProfileLoading.set(false);
        if (res.success && res.data) {
          this.user.set(res.data);
          this.closeProfileModal();
          this.successMessage.set(res.message || 'Podaci naloga su uspesno izmenjeni!');
          setTimeout(() => this.successMessage.set(null), 3500);
        }
      },
      error: (err) => {
        this.isProfileLoading.set(false);
        this.errorMessage.set(err.error?.error || 'Greska pri izmeni podataka naloga.');
      }
    });
  }

  logout(): void {
    this.authService.logout();
  }
}

