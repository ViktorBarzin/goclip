Name:           goclip
Version:        0.0.1
Release:        0
License:        MIT
Summary:        Share clipboard contents over a network
Url:            https://github.com/ViktorBarzin/goclip
Group:          Development/Languages/Other
Source:	    %{name}-%{version}.tar.gz
Requires:       gtk3-devel
Requires:       cairo-devel
Requires:       golang

%description
Share clipboard contents over a network

%prep
%autosetup -n %{name}-%{version}

%build
# set up temporary build gopath, and put our directory there
mkdir -p ./_build/src/github.com/%{name}
ln -s $(pwd) ./_build/src/github.com/%{name}/app
GO111MODULE=on go get -u github.com/viktorbarzin/%{name}
export GOPATH=$(pwd)/_build:%{gopath}
go build -o %{name} .

%install
install -d %{buildroot}%{_bindir}
install -p -m 0755 ./%{name} %{buildroot}%{_bindir}/%{name}

#%check
#%gotest $IMPORTPATH_NAMESPACE

%files 
%{_bindir}/%{name}
%doc README.md LICENSE

#%changelog

