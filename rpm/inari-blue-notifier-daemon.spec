Name:           inari-blue-notifier-daemon
Version:        1.3.0
Release:        1%{?dist}
Summary:        Blink your Inari Blue on incoming calls

License:        MIT
URL:            https://github.com/RikudouSage/InariBlueNotifier
Source0:        %{name}-%{version}.tar.gz

Requires:       systemd-user-session-targets
Requires:       symbiosis
BuildRequires:  systemd


%define debug_package %{nil}

%description
Blink your Inari Blue on incoming calls

%prep
%autosetup

%build
: # The binary is cross-compiled on the host by the Makefile.

%install
install -Dpm 0755 %{name} %{buildroot}%{_bindir}/%{name}
install -Dpm 0644 assets/toh-config.yaml %{buildroot}%{_datadir}/tohd-1/tohs/0001/0004/90-%{name}.yaml
install -Dpm 0644 assets/inari-blue-notifier-daemon.service %{buildroot}%{_userunitdir}/%{name}.service

%post
systemctl reload-or-try-restart symbiosis.service || :
systemctl-user daemon-reload || :
if [ "$1" -eq 1 ]; then
  systemctl-user enable --now %{name}.service || :
else
  systemctl-user try-restart %{name}.service || :
fi

%preun
if [ "$1" -eq 0 ]; then
  systemctl-user disable --now %{name}.service || :
fi

%postun
systemctl-user daemon-reload || :
if [ "$1" -eq 0 ]; then
    systemctl reload-or-try-restart symbiosis.service || :
fi

%files
%license LICENSE
%{_bindir}/%{name}
%{_datadir}/tohd-1/tohs/0001/0004/90-%{name}.yaml
%{_userunitdir}/%{name}.service
