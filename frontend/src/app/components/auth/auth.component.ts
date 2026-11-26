import { Component, OnInit, signal } from '@angular/core';
import { CommonModule } from '@angular/common';
import { FormsModule } from '@angular/forms';
import { ActivatedRoute, Router } from '@angular/router';
import { AuthService } from '../../services/auth.service';

type AuthTab = 'login' | 'register' | 'verify' | 'recovery';

@Component({
  selector: 'app-auth',
  standalone: true,
  imports: [CommonModule, FormsModule],
  templateUrl: './auth.component.html',
  styleUrl: './auth.component.css'
})
export class AuthComponent implements OnInit {
  currentTab = signal<AuthTab>('login');

  // Login polja
  loginEmail = '';
  loginPassword = '';
  loginOtpCode = '';
  loginStep = signal<'credentials' | 'otp'>('credentials');

  // Register polja
  regFirstName = '';
  regLastName = '';
  regEmail = '';
  regPassword = '';
  regConfirmPassword = '';

  // Verify polja
  verifyEmail = '';
  verifyCode = '';

  // Recovery polja (1.2 Zaboravljena lozinka & 1.3 Povracaj naloga magicnim linkom)
  recoveryMode = signal<'magic' | 'reset'>('magic');
  magicEmail = '';
  magicToken = '';
  magicStep = signal<'request' | 'verify'>('request');

  resetEmail = '';
  resetToken = '';
  resetNewPassword = '';
  resetConfirmPassword = '';
  resetStep = signal<'request' | 'confirm'>('request');

  // Poruke
  errorMessage = signal<string | null>(null);
  successMessage = signal<string | null>(null);
  isLoading = signal<boolean>(false);

  constructor(
    private authService: AuthService,
    private router: Router,
    private route: ActivatedRoute
  ) {
    if (this.authService.isLoggedIn()) {
      this.router.navigate(['/drive']);
    }
  }

  ngOnInit(): void {
    this.route.queryParams.subscribe(params => {
      if (params['magicEmail'] && params['magicToken']) {
        this.currentTab.set('recovery');
        this.recoveryMode.set('magic');
        this.magicEmail = params['magicEmail'];
        this.magicToken = params['magicToken'];
        this.magicStep.set('verify');
        this.onVerifyMagicLink();
      } else if (params['email'] && params['token']) {
        this.currentTab.set('recovery');
        this.recoveryMode.set('reset');
        this.resetEmail = params['email'];
        this.resetToken = params['token'];
        this.resetStep.set('confirm');
      }
    });
  }

  setTab(tab: AuthTab): void {
    this.currentTab.set(tab);
    this.clearMessages();
  }

  clearMessages(): void {
    this.errorMessage.set(null);
    this.successMessage.set(null);
  }

  // --- LOGIN ---
  onLoginSubmit(): void {
    this.clearMessages();

    // [US 09.02] Klijentska validacija ulaznih podataka pre slanja na server
    if (this.loginStep() === 'credentials') {
      const email = this.loginEmail.trim();
      const pass = this.loginPassword.trim();
      if (!email || !pass) {
        this.errorMessage.set('Obavezna polja ne mogu biti prazna: unesite email i lozinku.');
        return;
      }
      if (!/^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(email)) {
        this.errorMessage.set('Format email adrese nije validan.');
        return;
      }
    } else {
      if (!/^\d{6}$/.test(this.loginOtpCode.trim())) {
        this.errorMessage.set('OTP kod mora sadrzati tacno 6 cifara.');
        return;
      }
    }

    this.isLoading.set(true);

    if (this.loginStep() === 'credentials') {
      this.authService.login(this.loginEmail, this.loginPassword).subscribe({
        next: (res) => {
          this.isLoading.set(false);
          if (res.success && res.data?.step === 'OTP_REQUIRED') {
            this.loginStep.set('otp');
            this.successMessage.set('Lozinka ispravna! Proverite MailHog (http://localhost:8025) za jednokratni OTP kod.');
          }
        },
        error: (err) => {
          this.isLoading.set(false);
          this.errorMessage.set(err.error?.error || 'Greska pri prijavi na sistem.');
        }
      });
    } else {
      // Potvrda OTP koda
      this.authService.verifyOtp(this.loginEmail, this.loginOtpCode).subscribe({
        next: (res) => {
          this.isLoading.set(false);
          if (res.success) {
            this.router.navigate(['/drive']);
          }
        },
        error: (err) => {
          this.isLoading.set(false);
          this.errorMessage.set(err.error?.error || 'Neispravan ili istekao OTP kod.');
        }
      });
    }
  }

  resetLoginStep(): void {
    this.loginStep.set('credentials');
    this.loginOtpCode = '';
    this.clearMessages();
  }

  // --- REGISTRACIJA ---
  onRegisterSubmit(): void {
    this.clearMessages();

    // [US 09.02] Klijentska validacija ulaznih podataka (prazna polja, duzina, format, specijalni karakteri)
    const first = this.regFirstName.trim();
    const last = this.regLastName.trim();
    const email = this.regEmail.trim();
    if (!first || !last || !email || !this.regPassword || !this.regConfirmPassword) {
      this.errorMessage.set('Sva obavezna polja moraju biti popunjena.');
      return;
    }
    if (first.length < 2 || first.length > 50 || last.length < 2 || last.length > 50) {
      this.errorMessage.set('Ime i prezime moraju imati izmedju 2 i 50 karaktera.');
      return;
    }
    if (/[<>${}]/.test(first) || /[<>${}]/.test(last) || /[<>${}]/.test(email)) {
      this.errorMessage.set('Uneta polja sadrze nedozvoljene specijalne karaktere.');
      return;
    }
    if (!/^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(email)) {
      this.errorMessage.set('Format email adrese nije validan.');
      return;
    }
    if (this.regPassword !== this.regConfirmPassword) {
      this.errorMessage.set('Lozinka i ponovljena lozinka se ne poklapaju.');
      return;
    }
    if (
      this.regPassword.length < 8 ||
      !/[A-Z]/.test(this.regPassword) ||
      !/[a-z]/.test(this.regPassword) ||
      !/[0-9]/.test(this.regPassword) ||
      !/[^A-Za-z0-9]/.test(this.regPassword)
    ) {
      this.errorMessage.set('Lozinka mora imati bar 8 karaktera, veliko i malo slovo, broj i specijalan znak.');
      return;
    }

    this.isLoading.set(true);

    const payload = {
      firstName: this.regFirstName,
      lastName: this.regLastName,
      email: this.regEmail,
      password: this.regPassword,
      confirmPassword: this.regConfirmPassword
    };

    this.authService.register(payload).subscribe({
      next: (res) => {
        this.isLoading.set(false);
        this.successMessage.set(res.message || 'Registracija uspesna! Unesite kod sa mejla za potvrdu.');
        this.verifyEmail = this.regEmail;
        this.currentTab.set('verify');
      },
      error: (err) => {
        this.isLoading.set(false);
        this.errorMessage.set(err.error?.error || 'Greska pri registraciji.');
      }
    });
  }

  // --- POTVRDA NALOGA ---
  onVerifySubmit(): void {
    this.clearMessages();
    this.isLoading.set(true);

    this.authService.verifyEmail(this.verifyEmail, this.verifyCode).subscribe({
      next: (res) => {
        this.isLoading.set(false);
        this.successMessage.set(res.message || 'Nalog je uspesno potvrdjen! Mozete se ulogovati.');
        this.loginEmail = this.verifyEmail;
        this.currentTab.set('login');
      },
      error: (err) => {
        this.isLoading.set(false);
        this.errorMessage.set(err.error?.error || 'Neispravan verifikacioni kod.');
      }
    });
  }

  // --- 1.3 POVRACAJ NALOGA (MAGICNI LINK) ---
  setRecoveryMode(mode: 'magic' | 'reset'): void {
    this.recoveryMode.set(mode);
    this.clearMessages();
  }

  onRequestMagicLink(): void {
    this.clearMessages();
    this.isLoading.set(true);

    this.authService.requestMagicLink(this.magicEmail).subscribe({
      next: (res) => {
        this.isLoading.set(false);
        this.magicStep.set('verify');
        this.successMessage.set(res.message || 'Magicni link i token su poslati na email (proverite MailHog na :8025).');
      },
      error: (err) => {
        this.isLoading.set(false);
        this.errorMessage.set(err.error?.error || 'Greska pri slanju magicnog linka.');
      }
    });
  }

  onVerifyMagicLink(): void {
    this.clearMessages();
    this.isLoading.set(true);

    this.authService.verifyMagicLink(this.magicEmail, this.magicToken).subscribe({
      next: (res) => {
        this.isLoading.set(false);
        if (res.success) {
          this.router.navigate(['/drive']);
        }
      },
      error: (err) => {
        this.isLoading.set(false);
        this.errorMessage.set(err.error?.error || 'Neispravan ili istekao magicni link/token.');
      }
    });
  }

  // --- 1.2 ZABORAVLJENA LOZINKA (RESET LOZINKE) ---
  onForgotPasswordSubmit(): void {
    this.clearMessages();
    this.isLoading.set(true);

    this.authService.forgotPassword(this.resetEmail).subscribe({
      next: (res) => {
        this.isLoading.set(false);
        this.resetStep.set('confirm');
        this.successMessage.set(res.message || 'Kratkotrajni link i token za reset lozinke su poslati na email.');
      },
      error: (err) => {
        this.isLoading.set(false);
        this.errorMessage.set(err.error?.error || 'Greska pri zahtevu za reset lozinke.');
      }
    });
  }

  onResetPasswordSubmit(): void {
    this.clearMessages();
    this.isLoading.set(true);

    this.authService.resetPassword({
      email: this.resetEmail,
      token: this.resetToken,
      newPassword: this.resetNewPassword,
      confirmPassword: this.resetConfirmPassword
    }).subscribe({
      next: (res) => {
        this.isLoading.set(false);
        this.successMessage.set(res.message || 'Lozinka je uspesno promenjena! Mozete se prijaviti.');
        this.loginEmail = this.resetEmail;
        this.currentTab.set('login');
      },
      error: (err) => {
        this.isLoading.set(false);
        this.errorMessage.set(err.error?.error || 'Greska pri promeni lozinke.');
      }
    });
  }
}

