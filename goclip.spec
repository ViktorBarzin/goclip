#
# Copyright (c) 2019 SUSE LLC
#
# All modifications and additions to the file contributed by third parties
# remain the property of their copyright owners, unless otherwise agreed
# upon. The license for this file, and modifications and additions to the
# file, is the same license as for the pristine package itself (unless the
# license for the pristine package is not an Open Source License, in which
# case the license is the MIT License). An "Open Source License" is a
# license that conforms to the Open Source Definition (Version 1.9)
# published by the Open Source Initiative.

# Please submit bugfixes or comments via https://bugs.opensuse.org/
#

%{go_nostrip}

Name:           goclip
Version:        0.0.0+git20140916
Release:        0
License:        MIT
Summary:        Share clipboard contents over a network
Url:            https://github.com/ViktorBarzin/goclip
Group:          Development/Languages/Other
# Source:         $EXACT_UPSTREAM_NAME-%{version}.tar.xz
BuildRequires:  golang-packaging
BuildRequires:  xxx-devel
BuildRequires:  xz
Requires:       xxx-devel
Requires:       gtk3-devel
Requires:       cairo-devel
Requires:       glib-devel
%{go_provides}

%description
Share clipboard contents over a network

# %prep
# %setup -q -n $EXACT_UPSTREAM_NAME-%{version}

%build
%goprep github.com/viktorbarzin/goclip
%gobuild github.com/viktorbarzin/goclip

%install
%goinstall
%gosrc

%gofilelist

# %check
# %gotest $IMPORTPATH_NAMESPACE

%files -f file.lst
%doc README LICENSE

%changelog
