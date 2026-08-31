document.addEventListener('DOMContentLoaded', () => {
    const loginView = document.getElementById('login-view');
    const dashboardView = document.getElementById('dashboard-view');
    const btnLogin = document.getElementById('btn-login');
    const userNameEl = document.getElementById('user-name');
    const userAvatarEl = document.getElementById('user-avatar');
    
    // Sidebar elements
    const groupList = document.getElementById('group-list');
    const groupsLoading = document.getElementById('groups-loading');
    const groupSearch = document.getElementById('group-search');
    
    // Scan panel elements
    const scanForm = document.getElementById('scan-form');
    const noGroupSelected = document.getElementById('no-group-selected');
    const projectsList = document.getElementById('projects-list');
    const selectAllProjects = document.getElementById('select-all-projects');
    
    // Results elements
    const loadingState = document.getElementById('loading-state');
    const resultsPanel = document.getElementById('results-panel');
    const resultsBody = document.getElementById('results-body');
    const resultsTable = document.getElementById('results-table');
    const noResults = document.getElementById('no-results');
    const btnExport = document.getElementById('btn-export');

    let currentScanRequest = null;
    let lastScanResults = [];
    let selectedGroupIds = new Set();
    let selectedGroupNames = new Set();

    // Check if user is logged in
    fetch('/auth/me')
        .then(response => {
            if (response.ok) {
                return response.json();
            }
            throw new Error('Not authenticated');
        })
        .then(data => {
            // User is logged in
            userNameEl.textContent = data.username;
            if (data.avatar_url) {
                userAvatarEl.src = data.avatar_url;
            } else {
                userAvatarEl.src = `https://ui-avatars.com/api/?name=${data.username}&background=random`;
            }
            loginView.classList.remove('active');
            dashboardView.classList.add('active');
            
            // Fetch groups
            fetchGroups();
        })
        .catch(() => {
            // Not logged in, stay on login view
            loginView.classList.add('active');
            dashboardView.classList.remove('active');
        });

    btnLogin.addEventListener('click', () => {
        window.location.href = '/auth/login';
    });

    let allGroupsRaw = [];
    let groupsMap = {};
    let rootGroups = [];

    function fetchGroups() {
        fetch('/api/groups')
            .then(res => res.json())
            .then(data => {
                groupsLoading.classList.add('hidden');
                if (!data.groups || data.groups.length === 0) {
                    let errorMsg = data.error ? ` (Error: ${data.error})` : '';
                    groupList.innerHTML = `<li class="group-item" style="cursor:default; color: var(--color-destructive)">No groups found${errorMsg}</li>`;
                    return;
                }
                
                allGroupsRaw = data.groups;
                buildGroupTree();
                renderGroupTree(rootGroups, groupList);
            })
            .catch(err => {
                groupsLoading.textContent = 'Failed to load groups.';
                console.error(err);
            });
    }

    function buildGroupTree() {
        groupsMap = {};
        rootGroups = [];
        
        // Sort alphabetically by path
        allGroupsRaw.sort((a, b) => a.path.localeCompare(b.path));
        
        allGroupsRaw.forEach(g => {
            groupsMap[g.id] = { ...g, children: [], expanded: false };
        });
        
        allGroupsRaw.forEach(g => {
            if (g.parent_id && groupsMap[g.parent_id]) {
                groupsMap[g.parent_id].children.push(groupsMap[g.id]);
            } else {
                rootGroups.push(groupsMap[g.id]);
            }
        });
    }

    function renderGroupTree(nodes, container, level = 0, filterText = "") {
        if (level === 0) container.innerHTML = '';
        
        nodes.forEach(node => {
            // Apply filter
            const matchesFilter = node.full_path.toLowerCase().includes(filterText.toLowerCase());
            
            // Check if any children match
            let hasMatchingChildren = false;
            if (filterText) {
                const checkChildrenMatch = (n) => {
                    if (n.full_path.toLowerCase().includes(filterText.toLowerCase())) return true;
                    return n.children.some(c => checkChildrenMatch(c));
                };
                hasMatchingChildren = node.children.some(c => checkChildrenMatch(c));
            }

            if (filterText && !matchesFilter && !hasMatchingChildren) {
                return;
            }
            
            // Auto-expand if filtering
            if (filterText && hasMatchingChildren) {
                node.expanded = true;
            }

            const li = document.createElement('li');
            li.style.paddingLeft = `${level * 16 + 12}px`;
            li.className = 'group-item';
            if (selectedGroupIds.has(node.id)) li.classList.add('active');
            
            const contentDiv = document.createElement('div');
            contentDiv.className = 'group-item-content';
            
            const expander = document.createElement('span');
            expander.className = 'group-expander';
            if (node.children.length > 0) {
                expander.classList.add(node.expanded ? 'expanded' : 'collapsed');
                expander.addEventListener('click', (e) => {
                    e.stopPropagation();
                    node.expanded = !node.expanded;
                    renderGroupTree(rootGroups, groupList, 0, groupSearch.value.trim());
                });
            } else {
                expander.classList.add('empty');
            }
            
            const checkbox = document.createElement('input');
            checkbox.type = 'checkbox';
            checkbox.className = 'group-checkbox';
            checkbox.checked = selectedGroupIds.has(node.id);
            checkbox.addEventListener('click', (e) => {
                e.stopPropagation();
                if (checkbox.checked) {
                    selectedGroupIds.add(node.id);
                    selectedGroupNames.add(node.full_path);
                    li.classList.add('active');
                } else {
                    selectedGroupIds.delete(node.id);
                    selectedGroupNames.delete(node.full_path);
                    li.classList.remove('active');
                }
                fetchMultipleGroupProjects();
            });

            const nameSpan = document.createElement('span');
            nameSpan.className = 'group-name';
            nameSpan.textContent = node.name; // Short name for tree view
            nameSpan.title = node.full_path;
            
            contentDiv.appendChild(expander);
            contentDiv.appendChild(checkbox);
            contentDiv.appendChild(nameSpan);
            li.appendChild(contentDiv);
            
            li.addEventListener('click', (e) => {
                if (e.target !== expander && e.target !== checkbox) {
                    if (node.children.length > 0) {
                        node.expanded = !node.expanded;
                        renderGroupTree(rootGroups, groupList, 0, groupSearch ? groupSearch.value.trim() : "");
                    }
                }
            });
            
            container.appendChild(li);
            
            if (node.expanded && node.children.length > 0) {
                const childrenContainer = document.createElement('ul');
                childrenContainer.style.listStyle = 'none';
                childrenContainer.style.padding = '0';
                renderGroupTree(node.children, childrenContainer, level + 1, filterText);
                container.appendChild(childrenContainer);
            }
        });
    }

    if (groupSearch) {
        groupSearch.addEventListener('input', (e) => {
            renderGroupTree(rootGroups, groupList, 0, e.target.value.trim());
        });
    }

    function fetchMultipleGroupProjects() {
        if (selectedGroupIds.size === 0) {
            noGroupSelected.classList.remove('hidden');
            scanForm.classList.add('hidden');
            projectsList.innerHTML = '';
            return;
        }

        noGroupSelected.classList.add('hidden');
        scanForm.classList.remove('hidden');
        
        projectsList.innerHTML = '<div style="padding: 1rem; color: var(--color-muted-foreground);">Loading repositories...</div>';
        selectAllProjects.checked = false;
        
        const promises = Array.from(selectedGroupIds).map(id => 
            fetch(`/api/groups/${id}/projects`).then(res => res.json())
        );

        Promise.all(promises)
            .then(results => {
                let allProjects = [];
                results.forEach(data => {
                    if (data.projects) {
                        allProjects = allProjects.concat(data.projects);
                    }
                });

                projectsList.innerHTML = '';
                if (allProjects.length === 0) {
                    projectsList.innerHTML = '<div style="padding: 1rem; color: var(--color-muted-foreground);">No repositories found in selected groups.</div>';
                    return;
                }
                
                // Deduplicate by ID
                const uniqueProjects = Array.from(new Map(allProjects.map(p => [p.id, p])).values());
                uniqueProjects.sort((a, b) => a.path_with_namespace.localeCompare(b.path_with_namespace));
                
                uniqueProjects.forEach(project => {
                    const div = document.createElement('div');
                    div.className = 'project-item';
                    
                    const pCheckbox = document.createElement('input');
                    pCheckbox.type = 'checkbox';
                    pCheckbox.className = 'project-checkbox';
                    pCheckbox.value = project.id;
                    pCheckbox.id = `proj-${project.id}`;
                    
                    const label = document.createElement('label');
                    label.htmlFor = `proj-${project.id}`;
                    label.textContent = project.path_with_namespace;
                    
                    pCheckbox.addEventListener('change', updateSelectAllState);
                    
                    div.appendChild(pCheckbox);
                    div.appendChild(label);
                    projectsList.appendChild(div);
                });
            })
            .catch(err => {
                projectsList.innerHTML = '<div style="padding: 1rem; color: var(--color-destructive);">Failed to load repositories.</div>';
                console.error(err);
            });
    }

    function updateSelectAllState() {
        const checkboxes = document.querySelectorAll('.project-checkbox');
        const allChecked = Array.from(checkboxes).every(c => c.checked);
        selectAllProjects.checked = checkboxes.length > 0 && allChecked;
    }

    selectAllProjects.addEventListener('change', (e) => {
        const checkboxes = document.querySelectorAll('.project-checkbox');
        checkboxes.forEach(c => c.checked = e.target.checked);
    });

    scanForm.addEventListener('submit', (e) => {
        e.preventDefault();
        
        const checkboxes = document.querySelectorAll('.project-checkbox:checked');
        if (checkboxes.length === 0) {
            alert('Please select at least one repository to scan.');
            return;
        }
        
        const projectIds = Array.from(checkboxes).map(c => parseInt(c.value));
        const keywords = document.getElementById('keywords').value.trim();
        const branch = document.getElementById('branch').value.trim();
        
        if (!keywords) return;

        currentScanRequest = { 
            project_ids: projectIds, 
            keywords: keywords, 
            branch: branch, 
            group_id: Array.from(selectedGroupNames).join(', ') 
        };

        // UI transitions
        scanForm.querySelector('button').disabled = true;
        loadingState.classList.remove('hidden');
        resultsPanel.classList.add('hidden');
        resultsBody.innerHTML = '';

        fetch('/api/scan', {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json'
            },
            body: JSON.stringify({
                project_ids: projectIds,
                keywords: keywords,
                branch: branch
            })
        })
        .then(response => {
            if (!response.ok) {
                return response.json().then(errData => {
                    throw new Error(errData.error || 'Network response was not ok');
                }).catch(e => {
                    throw new Error(e.message || 'Network response was not ok');
                });
            }
            return response.json();
        })
        .then(data => {
            lastScanResults = data.results || [];
            scanForm.querySelector('button').disabled = false;
            loadingState.classList.add('hidden');
            resultsPanel.classList.remove('hidden');
            
            renderResults(lastScanResults);
        })
        .catch(error => {
            scanForm.querySelector('button').disabled = false;
            loadingState.classList.add('hidden');
            alert('Scan failed: ' + error.message);
        });
    });

    function renderResults(results) {
        if (results.length === 0) {
            resultsTable.classList.add('hidden');
            noResults.classList.remove('hidden');
            btnExport.disabled = true;
            return;
        }

        resultsTable.classList.remove('hidden');
        noResults.classList.add('hidden');
        btnExport.disabled = false;
        resultsBody.innerHTML = '';

        results.forEach(res => {
            const tr = document.createElement('tr');
            
            tr.innerHTML = `
                <td>
                    <strong>${res.project_name}</strong>
                </td>
                <td>
                    <span style="color: var(--color-accent); font-weight: 500;">${res.keyword_found}</span>
                </td>
                <td>
                    <span class="file-path">${res.filename}</span>
                    <div class="code-snippet">${escapeHtml(res.line_content)}</div>
                </td>
                <td>
                    <a href="${res.deep_link}" target="_blank" rel="noopener noreferrer" class="action-link">
                        View File
                        <svg viewBox="0 0 24 24" width="16" height="16" stroke="currentColor" stroke-width="2" fill="none" stroke-linecap="round" stroke-linejoin="round"><path d="M18 13v6a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2V8a2 2 0 0 1 2-2h6"></path><polyline points="15 3 21 3 21 9"></polyline><line x1="10" y1="14" x2="21" y2="3"></line></svg>
                    </a>
                </td>
            `;
            resultsBody.appendChild(tr);
        });
    }

    btnExport.addEventListener('click', () => {
        if (!currentScanRequest) return;

        // Change button state
        const originalText = btnExport.innerHTML;
        btnExport.innerHTML = 'Exporting...';
        btnExport.disabled = true;

        fetch('/api/export', {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json'
            },
            body: JSON.stringify({
                group_id: currentScanRequest.group_id.replace(/\//g, '_'),
                results: lastScanResults
            })
        })
        .then(response => {
            if (!response.ok) throw new Error('Export failed');
            return response.blob();
        })
        .then(blob => {
            const url = window.URL.createObjectURL(blob);
            const a = document.createElement('a');
            a.href = url;
            a.download = `scan_report_projects.pdf`;
            document.body.appendChild(a);
            a.click();
            window.URL.revokeObjectURL(url);
            a.remove();
        })
        .catch(error => {
            alert(error.message);
        })
        .finally(() => {
            btnExport.innerHTML = originalText;
            btnExport.disabled = false;
        });
    });

    function escapeHtml(unsafe) {
        return (unsafe || "")
             .replace(/&/g, "&amp;")
             .replace(/</g, "&lt;")
             .replace(/>/g, "&gt;")
             .replace(/"/g, "&quot;")
             .replace(/'/g, "&#039;");
    }
});
