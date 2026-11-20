import { Routes } from '@angular/router';
import { AuthComponent } from './components/auth/auth.component';
import { DriveComponent } from './components/drive/drive.component';
import { authGuard } from './guards/auth.guard';

export const routes: Routes = [
  { path: 'auth', component: AuthComponent },
  { path: 'drive', component: DriveComponent, canActivate: [authGuard] },
  { path: '', redirectTo: 'drive', pathMatch: 'full' },
  { path: '**', redirectTo: 'drive' }
];
