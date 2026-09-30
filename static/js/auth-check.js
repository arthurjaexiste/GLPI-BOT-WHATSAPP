async function checkAuthAndRole() {
    try {
        const res = await fetch('/api/me');
        if (!res.ok) {
            if (res.status === 401 && window.location.pathname !== '/login') {
                window.location.href = '/login';
            }
            return;
        }
        const user = await res.json();
        
        const badgeName = document.getElementById('user-badge-name');
        const badgeRole = document.getElementById('user-badge-role');
        const badgeContainer = document.getElementById('user-badge-header');

        if (badgeContainer) badgeContainer.classList.remove('hidden');
        if (badgeName) badgeName.innerText = user.name || user.username;
        if (badgeRole) {
            if (user.role === 'admin') {
                badgeRole.innerHTML = `
                    <span class="inline-flex items-center gap-1">
                        <svg class="w-3 h-3 text-purple-400 shrink-0" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 12l2 2 4-4m5.618-4.016A11.955 11.955 0 0112 2.944a11.955 11.955 0 01-8.618 3.04A12.02 12.02 0 003 9c0 5.591 3.824 10.29 9 11.622 5.176-1.332 9-6.03 9-11.622 0-1.042-.133-2.052-.382-3.016z"/></svg>
                        <span>Admin</span>
                    </span>
                `;
                badgeRole.className = 'inline-flex items-center px-2 py-0.5 rounded text-[10px] font-bold uppercase tracking-wider bg-purple-950/80 text-purple-300 border border-purple-800/80 shadow-sm';
            } else {
                badgeRole.innerHTML = `
                    <span class="inline-flex items-center gap-1">
                        <svg class="w-3 h-3 text-sky-400 shrink-0" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M16 7a4 4 0 11-8 0 4 4 0 018 0zM12 14a7 7 0 00-7 7h14a7 7 0 00-7-7z"/></svg>
                        <span>Operador</span>
                    </span>
                `;
                badgeRole.className = 'inline-flex items-center px-2 py-0.5 rounded text-[10px] font-bold uppercase tracking-wider bg-sky-950/80 text-sky-300 border border-sky-800/80 shadow-sm';
            }
        }

        if (user.role === 'operator') {
            const adminSelectors = ['a[href="/flow"]', 'a[href="/config"]', 'a[href="/messages"]', 'a[href="/logs"]'];
            adminSelectors.forEach(sel => {
                document.querySelectorAll(sel).forEach(el => el.remove());
            });

            const currentPath = window.location.pathname;
            if (['/config', '/flow', '/messages', '/logs'].includes(currentPath)) {
                window.location.href = '/chats';
            }
        }
    } catch (e) {
        console.error("Erro ao verificar autenticação do usuário:", e);
    }
}

document.addEventListener('DOMContentLoaded', checkAuthAndRole);
