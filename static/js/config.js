// Função auxiliar de sanitização HTML
function escapeHTML(str) {
    if (str === null || str === undefined) return '';
    return String(str)
        .replace(/&/g, '&amp;')
        .replace(/</g, '&lt;')
        .replace(/>/g, '&gt;')
        .replace(/"/g, '&quot;')
        .replace(/'/g, '&#039;');
}

// Carrega as configurações atuais da API
// Função fetchConfig manipula a rotina correspondente na interface do painel
async function fetchConfig() {
    try {
        const response = await fetch('/api/config');
        if (!response.ok) throw new Error('Falha ao obter configurações');
        const cfg = await response.json();

        document.getElementById('company_name').value = cfg.company_name || '';
        document.getElementById('telefone_notificacao').value = cfg.telefone_notificacao || '';
        document.getElementById('glpi_api_url').value = cfg.glpi_api_url || '';
        document.getElementById('glpi_app_token').value = cfg.glpi_app_token || '';
        document.getElementById('glpi_user_token').value = cfg.glpi_user_token || '';
        document.getElementById('dark_list').value = cfg.dark_list || '';
        document.getElementById('support_agents').value = cfg.support_agents || '';

        // SMTP
        document.getElementById('smtp_enabled').checked = cfg.smtp_enabled || false;
        document.getElementById('smtp_host').value = cfg.smtp_host || '';
        document.getElementById('smtp_port').value = cfg.smtp_port || 587;
        document.getElementById('smtp_username').value = cfg.smtp_username || '';
        document.getElementById('smtp_password').value = cfg.smtp_password || '';
        document.getElementById('smtp_sender').value = cfg.smtp_sender || '';
        document.getElementById('smtp_receiver').value = cfg.smtp_receiver || '';

        // Horários e Dias de Trabalho
        document.getElementById('working_hours_enabled').checked = cfg.working_hours_enabled || false;
        document.getElementById('working_hours_start').value = cfg.working_hours_start || '08:00';
        document.getElementById('working_hours_end').value = cfg.working_hours_end || '18:00';
        document.getElementById('msg_ausencia').value = cfg.msg_ausencia || '';

        // Marca os checkboxes dos dias de trabalho
        const workingDays = cfg.working_days ? cfg.working_days.split(',') : ['1', '2', '3', '4', '5'];
        const checkboxes = document.getElementsByName('working_days_checkbox');
        checkboxes.forEach(cb => {
            cb.checked = workingDays.includes(cb.value);
        });
    } catch (err) {
        showToast('Erro ao obter as configurações.', '❌');
    }
}

// Salva as configurações via POST na API
// Função saveConfig manipula a rotina correspondente na interface do painel
async function saveConfig() {
    const company_name = document.getElementById('company_name').value.trim();
    const telefone_notificacao = document.getElementById('telefone_notificacao').value.trim();
    const glpi_api_url = document.getElementById('glpi_api_url').value.trim();
    const glpi_app_token = document.getElementById('glpi_app_token').value.trim();
    const glpi_user_token = document.getElementById('glpi_user_token').value.trim();
    const dark_list = document.getElementById('dark_list').value.trim();
    const support_agents = document.getElementById('support_agents').value.trim();

    // SMTP
    const smtp_enabled = document.getElementById('smtp_enabled').checked;
    const smtp_host = document.getElementById('smtp_host').value.trim();
    const smtp_port = parseInt(document.getElementById('smtp_port').value) || 587;
    const smtp_username = document.getElementById('smtp_username').value.trim();
    const smtp_password = document.getElementById('smtp_password').value.trim();
    const smtp_sender = document.getElementById('smtp_sender').value.trim();
    const smtp_receiver = document.getElementById('smtp_receiver').value.trim();

    // Horários e Dias de Trabalho
    const working_hours_enabled = document.getElementById('working_hours_enabled').checked;
    const working_hours_start = document.getElementById('working_hours_start').value.trim();
    const working_hours_end = document.getElementById('working_hours_end').value.trim();
    const msg_ausencia = document.getElementById('msg_ausencia').value.trim();

    // Dias de Trabalho selecionados
    const selectedDays = [];
    const checkboxes = document.getElementsByName('working_days_checkbox');
    checkboxes.forEach(cb => {
        if (cb.checked) {
            selectedDays.push(cb.value);
        }
    });
    const working_days = selectedDays.join(',');

    if (!glpi_api_url) {
        showToast('O Link da API do GLPI é obrigatório.', '⚠️');
        return;
    }

    const payload = {
        company_name,
        telefone_notificacao,
        glpi_api_url,
        glpi_app_token,
        glpi_user_token,
        dark_list,
        support_agents,
        smtp_enabled,
        smtp_host,
        smtp_port,
        smtp_username,
        smtp_password,
        smtp_sender,
        smtp_receiver,
        working_hours_enabled,
        working_hours_start,
        working_hours_end,
        working_days,
        msg_ausencia
    };

    try {
        const response = await fetch('/api/config', {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json'
            },
            body: JSON.stringify(payload)
        });

        if (!response.ok) throw new Error('Erro ao salvar as configurações');
        showToast('Configurações salvas e aplicadas com sucesso!', '✅');
    } catch (err) {
        showToast('Erro ao salvar configurações.', '❌');
    }
}

// Alterna entre mostrar e esconder a senha (tipo password / text)

// Função toggleVisibility manipula a rotina correspondente na interface do painel
function toggleVisibility(btn, id) {
    const input = document.getElementById(id);
    if (input.type === "password") {
        input.type = "text";
        btn.innerText = "🔒";
        btn.title = "Esconder Token";
    } else {
        input.type = "password";
        btn.innerText = "👁️";
        btn.title = "Visualizar Token";
    }
}

// Copia o valor do token para a área de transferência
// Função copyToClipboard manipula a rotina correspondente na interface do painel
async function copyToClipboard(id) {
    const input = document.getElementById(id);
    const value = input.value;
    if (!value) {
        showToast('Nada para copiar.', '⚠️');
        return;
    }
    try {
        await navigator.clipboard.writeText(value);
        showToast('Token copiado para a área de transferência!', '✅');
    } catch (err) {
        showToast('Erro ao copiar token.', '❌');
    }
}

// Exibe um toast temporário

// Função showToast manipula a rotina correspondente na interface do painel
function showToast(message, icon = '✅') {
    const toast = document.getElementById('toast');
    const toastIcon = document.getElementById('toast-icon');
    const toastMsg = document.getElementById('toast-message');

    toastIcon.innerText = icon;
    toastMsg.innerText = message;

    // Reset layout classes to avoid duplicate accumulation
    const baseClasses = "fixed bottom-6 right-6 px-5 py-3 rounded-lg shadow-2xl backdrop-blur-md transform transition duration-300 flex items-center gap-3 z-50";

    if (icon === '✅') {
        toast.className = baseClasses + " bg-emerald-950/80 border border-emerald-500/30 text-emerald-300";
    } else {
        toast.className = baseClasses + " bg-red-950/80 border border-red-500/30 text-red-300";
    }

    // Force transition by removing classes in a new microtask
    setTimeout(() => {
        toast.classList.remove('translate-y-24', 'opacity-0');
    }, 10);

    setTimeout(() => {
        toast.classList.add('translate-y-24', 'opacity-0');
    }, 3000);
}

// Altera a senha do administrador
// Função changePassword manipula a rotina correspondente na interface do painel
async function changePassword(event) {
    event.preventDefault();
    const current_password = document.getElementById('current_password').value;
    const new_password = document.getElementById('new_password').value;
    const confirm_password = document.getElementById('confirm_password').value;

    if (new_password !== confirm_password) {
        showToast('A nova senha e a confirmação não coincidem!', '⚠️');
        return;
    }

    try {
        const response = await fetch('/api/change-password', {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json'
            },
            body: JSON.stringify({ current_password, new_password })
        });

        if (!response.ok) {
            const errMsg = await response.text();
            throw new Error(errMsg || 'Erro ao alterar a senha');
        }

        showToast('Senha alterada com sucesso!', '✅');
        document.getElementById('change-password-form').reset();
    } catch (err) {
        showToast(err.message || 'Erro ao alterar a senha.', '❌');
    }
}

// Reinicia o bot
// Função restartBot manipula a rotina correspondente na interface do painel
async function restartBot() {
    if (!confirm('Tem certeza de que deseja reiniciar o bot? O painel ficará temporariamente indisponível por alguns segundos.')) {
        return;
    }

    try {
        const response = await fetch('/api/restart', {
            method: 'POST'
        });

        if (!response.ok) throw new Error('Falha ao enviar sinal de reinício');

        showToast('Sinal de reinício enviado! Aguardando o bot voltar...', '✅');

        // Bloqueia a tela informando que está reiniciando
        setTimeout(() => {
            document.body.innerHTML = `
                <div class="min-h-screen flex items-center justify-center bg-[#0b0f19] text-white flex-col gap-4">
                    <span class="text-5xl animate-spin text-indigo-500">🔄</span>
                    <h2 class="text-xl font-bold animate-pulse">Reiniciando o Sistema...</h2>
                    <p class="text-xs text-gray-400" id="restart-msg">Aguardando o bot subir novamente...</p>
                </div>
            `;

            // Inicia tentativa de reconexão automática após 4 segundos
            setTimeout(autoReconnect, 4000);
        }, 1000);
    } catch (err) {
        showToast('Erro ao reiniciar o bot.', '❌');
    }
}

// Tenta se reconectar ao bot em loop até o painel voltar, então redireciona para a home
// Função autoReconnect manipula a rotina correspondente na interface do painel
async function autoReconnect() {
    const msgEl = document.getElementById('restart-msg');
    try {
        // Tenta buscar o status da API
        const res = await fetch('/api/status');
        if (res.ok) {
            if (msgEl) msgEl.innerText = "Conectado! Redirecionando...";
            setTimeout(() => {
                window.location.href = '/';
            }, 1000);
            return;
        }
    } catch (err) {
        // Ignora erros de rede enquanto o servidor estiver fora do ar
        console.log("Servidor ainda offline, tentando novamente...");
    }

    // Tenta novamente após 1.5 segundos
    setTimeout(autoReconnect, 1500);
}

// Inicializa buscando as configurações
window.addEventListener('DOMContentLoaded', fetchConfig);

// Testar conexão SMTP antes de salvar
// Função testSMTP manipula a rotina correspondente na interface do painel
async function testSMTP() {
    const company_name = document.getElementById('company_name').value.trim();
    const smtp_host = document.getElementById('smtp_host').value.trim();
    const smtp_port = parseInt(document.getElementById('smtp_port').value) || 587;
    const smtp_username = document.getElementById('smtp_username').value.trim();
    const smtp_password = document.getElementById('smtp_password').value;
    const smtp_sender = document.getElementById('smtp_sender').value.trim();
    const smtp_receiver = document.getElementById('smtp_receiver').value.trim();

    if (!smtp_host || !smtp_username || !smtp_password || !smtp_sender || !smtp_receiver) {
        showToast('Preencha todos os campos do SMTP antes de testar.', '⚠️');
        return;
    }

    const btn = document.getElementById('btn-test-smtp');
    if (!btn) return;

    const originalText = btn.innerText;
    btn.disabled = true;
    btn.innerText = "⏳ Testando...";

    const payload = {
        company_name,
        smtp_host,
        smtp_port,
        smtp_username,
        smtp_password,
        smtp_sender,
        smtp_receiver
    };

    try {
        const response = await fetch('/api/config/test-smtp', {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json'
            },
            body: JSON.stringify(payload)
        });

        let data;
        try {
            data = await response.json();
        } catch (e) {
            data = { status: 'error', message: 'Servidor retornou resposta inválida' };
        }

        btn.disabled = false;
        btn.innerText = originalText;

        if (response.ok && data.status === 'ok') {
            showToast('E-mail de teste enviado com sucesso! Verifique a caixa de entrada.', '✅');
        } else {
            showToast('Falha no envio: ' + (data.message || 'Erro desconhecido'), '❌');
        }
    } catch (err) {
        btn.disabled = false;
        btn.innerText = originalText;
        console.error(err);
        showToast('Erro de conexão ao testar o SMTP.', '❌');
    }
}

// Alterna a exibição das abas do painel de configurações
function showTab(tabName) {
    // Esconde todos os painéis
    const panels = document.querySelectorAll('.settings-panel');
    panels.forEach(p => p.classList.add('hidden'));

    // Exibe o painel selecionado
    const activePanel = document.getElementById('panel-' + tabName);
    if (activePanel) {
        activePanel.classList.remove('hidden');
    }

    // Reseta o estilo de todas as abas
    const tabs = document.querySelectorAll('.tab-btn');
    tabs.forEach(t => {
        t.className = "tab-btn px-4 py-3 text-left text-xs font-bold rounded-xl transition-all duration-200 flex items-center gap-3 whitespace-nowrap text-zinc-400 hover:text-white hover:bg-white/5 border border-transparent";
    });

    // Aplica estilo ativo na aba selecionada
    const activeTab = document.getElementById('tab-' + tabName);
    if (activeTab) {
        activeTab.className = "tab-btn px-4 py-3 text-left text-xs font-bold rounded-xl transition-all duration-200 flex items-center gap-3 whitespace-nowrap text-white bg-zinc-800/40 border border-zinc-700/50 shadow-md active";
    }

    if (tabName === 'usuarios') {
        fetchSystemUsers();
    }
}

// ─── GESTÃO DE USUÁRIOS DO SISTEMA & MODAL DE IMPORTAÇÃO GLPI ───────────────────────────

let systemUsersList = [];
let allGLPIUsers = [];

// Carrega usuários cadastrados no sistema (Tabela Principal)
async function fetchSystemUsers() {
    const tbody = document.getElementById('users-table-body');
    if (!tbody) return;

    try {
        const response = await fetch('/api/users');
        if (!response.ok) {
            const errorMsg = await response.text();
            tbody.innerHTML = `<tr><td colspan="5" class="py-6 text-center text-rose-400 font-medium italic">⚠️ ${escapeHTML(errorMsg || "Erro ao carregar usuários do sistema.")}</td></tr>`;
            return;
        }
        systemUsersList = await response.json();
        renderSystemUsersTable(systemUsersList);
    } catch (err) {
        tbody.innerHTML = `<tr><td colspan="5" class="py-6 text-center text-rose-400 font-medium italic">⚠️ Erro de comunicação: ${escapeHTML(err.message)}</td></tr>`;
    }
}

function filterSystemUsers() {
    const searchInput = document.getElementById('system-user-search');
    if (!searchInput) return;
    const query = searchInput.value.toLowerCase().trim();
    if (!query) {
        renderSystemUsersTable(systemUsersList);
        return;
    }
    const filtered = systemUsersList.filter(u => 
        (u.name && u.name.toLowerCase().includes(query)) || 
        (u.username && u.username.toLowerCase().includes(query))
    );
    renderSystemUsersTable(filtered);
}

function renderSystemUsersTable(users) {
    const tbody = document.getElementById('users-table-body');
    if (!tbody) return;

    if (!users || users.length === 0) {
        tbody.innerHTML = `<tr><td colspan="5" class="py-6 text-center text-zinc-500 italic">Nenhum usuário cadastrado no sistema ainda. Clique em "📥 Importar Usuário do GLPI" acima.</td></tr>`;
        return;
    }

    let html = '';
    users.forEach((u, index) => {
        const isChecked = u.enabled ? 'checked' : '';
        const statusBadge = u.enabled 
            ? `<span class="inline-flex items-center gap-1.5 px-2.5 py-0.5 rounded-full text-[10px] font-bold bg-emerald-500/20 text-emerald-300 border border-emerald-500/30">🟢 Acesso Liberado</span>`
            : `<span class="inline-flex items-center gap-1.5 px-2.5 py-0.5 rounded-full text-[10px] font-bold bg-rose-500/20 text-rose-300 border border-rose-500/30">🔴 Acesso Bloqueado</span>`;

        const roleSelect = `
            <select id="user-role-select-${index}" onchange="updateSystemUserPermissions(${index})" class="bg-black/60 border border-white/10 text-xs p-1.5 rounded-xl text-zinc-200 focus:outline-none">
                <option value="operator" ${u.role === 'operator' ? 'selected' : ''}>👤 Operador</option>
                <option value="admin" ${u.role === 'admin' ? 'selected' : ''}>⭐ Administrador</option>
            </select>
        `;

        html += `
            <tr class="hover:bg-white/[0.02] transition">
                <td class="py-3 font-semibold text-zinc-200">${escapeHTML(u.name || u.username)}</td>
                <td class="py-3 font-mono text-zinc-400">@${escapeHTML(u.username)}</td>
                <td class="py-3">
                    <div class="flex items-center gap-3">
                        <label class="relative inline-flex items-center cursor-pointer">
                            <input type="checkbox" id="user-toggle-${index}" ${isChecked} onchange="updateSystemUserPermissions(${index})" class="sr-only peer">
                            <div class="w-9 h-5 bg-zinc-800 peer-focus:outline-none rounded-full peer peer-checked:after:translate-x-full peer-checked:after:border-white after:content-[''] after:absolute after:top-[2px] after:left-[2px] after:bg-zinc-400 after:rounded-full after:h-4 after:w-4 after:transition-all peer-checked:bg-white"></div>
                        </label>
                        ${statusBadge}
                    </div>
                </td>
                <td class="py-3">${roleSelect}</td>
                <td class="py-3 text-right flex items-center justify-end gap-2">
                    <button onclick="saveSystemUserDirect(${index})" class="px-3 py-1.5 text-xs font-bold text-amber-300 hover:text-amber-200 bg-amber-500/10 hover:bg-amber-500/20 border border-amber-500/20 rounded-xl shadow-sm transition-all duration-200 flex items-center gap-1.5 cursor-pointer" title="Salvar Alterações do Usuário">
                        <span>💾</span> <span>Salvar</span>
                    </button>
                    <button onclick="openSetPasswordModal('${escapeHTML(u.username)}', ${u.id}, '${escapeHTML(u.name || u.username)}')" class="px-3 py-1.5 text-xs font-semibold text-zinc-300 hover:text-white bg-white/5 hover:bg-white/10 border border-white/10 rounded-xl shadow-sm transition-all duration-200 flex items-center gap-1.5 cursor-pointer" title="Definir Senha de Acesso">
                        <span>🔑</span> <span>Senha</span>
                    </button>
                    <button onclick="deleteSystemUser('${escapeHTML(u.username)}')" class="px-2.5 py-1.5 text-xs font-semibold text-rose-400 hover:text-rose-300 bg-rose-500/10 hover:bg-rose-500/20 border border-rose-500/20 rounded-xl transition-all duration-200 flex items-center gap-1 cursor-pointer" title="Remover do Sistema">
                        <span>🗑️</span>
                    </button>
                </td>
            </tr>
        `;
    });

    tbody.innerHTML = html;
}

function openSetPasswordModal(username, userId, name) {
    const modal = document.getElementById('modal-set-password');
    const idInput = document.getElementById('modal-password-user-id');
    const userInput = document.getElementById('modal-password-username');
    const subtitle = document.getElementById('modal-password-subtitle');
    const passInput = document.getElementById('modal-new-password');

    if (idInput) idInput.value = userId || '';
    if (userInput) userInput.value = username || '';
    if (subtitle) subtitle.innerText = `Definindo senha para @${username} (${name || username})`;
    if (passInput) passInput.value = '';

    if (modal) modal.classList.remove('hidden');
}

function closeSetPasswordModal() {
    const modal = document.getElementById('modal-set-password');
    if (modal) modal.classList.add('hidden');
}

async function saveUserPasswordFromModal(event) {
    event.preventDefault();
    const userId = parseInt(document.getElementById('modal-password-user-id').value) || 0;
    const username = document.getElementById('modal-password-username').value;
    const password = document.getElementById('modal-new-password').value;

    if (!password) {
        showToast('A senha não pode ser vazia.', '⚠️');
        return;
    }

    try {
        const response = await fetch('/api/users', {
            method: 'PUT',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({
                id: userId,
                username: username,
                name: username,
                password: password,
                role: 'operator'
            })
        });

        if (!response.ok) {
            const errMsg = await response.text();
            throw new Error(errMsg || 'Erro ao definir senha');
        }

        showToast(`Senha para @${username} definida com sucesso!`, '✅');
        closeSetPasswordModal();
    } catch (err) {
        showToast(err.message || 'Erro ao salvar senha.', '❌');
    }
}

async function updateSystemUserPermissions(index) {
    const user = systemUsersList[index];
    if (!user) return;

    const toggle = document.getElementById(`user-toggle-${index}`);
    const roleSelect = document.getElementById(`user-role-select-${index}`);

    if (toggle) user.enabled = toggle.checked;
    if (roleSelect) user.role = roleSelect.value;

    renderSystemUsersTable(systemUsersList);
    await saveSystemUserDirect(index, true);
}

async function saveSystemUserDirect(index, silent = false) {
    const user = systemUsersList[index];
    if (!user) return;

    const payload = {
        glpi_id: user.glpi_id || 0,
        username: user.username,
        name: user.name,
        role: user.role,
        enabled: user.enabled
    };

    try {
        const response = await fetch('/api/users/toggle-access', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify(payload)
        });

        if (!response.ok) {
            const errMsg = await response.text();
            throw new Error(errMsg || 'Erro ao salvar permissão');
        }

        if (!silent) {
            showToast(`Permissão para @${user.username} salva com sucesso!`, '✅');
        }
    } catch (err) {
        showToast(err.message || 'Erro ao atualizar permissão do usuário.', '❌');
    }
}

async function deleteSystemUser(username) {
    if (!confirm(`Tem certeza que deseja remover o usuário @${username} do sistema?`)) return;

    try {
        const response = await fetch('/api/users/delete', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ username })
        });

        if (!response.ok) {
            const errMsg = await response.text();
            throw new Error(errMsg || 'Erro ao remover usuário');
        }

        showToast(`Usuário @${username} removido com sucesso.`, '✅');
        fetchSystemUsers();
    } catch (err) {
        showToast(err.message || 'Erro ao remover usuário.', '❌');
    }
}

// ─── MODAL DE IMPORTAÇÃO DE USUÁRIOS DO GLPI ───────────────────────────

function openImportGLPIModal() {
    const modal = document.getElementById('modal-import-glpi');
    if (modal) {
        modal.classList.remove('hidden');
    }
    syncModalGLPIUsers();
}

function closeImportGLPIModal() {
    const modal = document.getElementById('modal-import-glpi');
    if (modal) {
        modal.classList.add('hidden');
    }
}

async function syncModalGLPIUsers() {
    const listContainer = document.getElementById('modal-glpi-user-list');
    const btnSync = document.getElementById('btn-modal-sync-glpi');
    if (!listContainer) return;

    if (btnSync) {
        btnSync.disabled = true;
        btnSync.innerText = '⏳ Conectando...';
    }

    listContainer.innerHTML = `
        <div class="py-12 text-center text-zinc-400 text-xs italic flex flex-col items-center gap-2">
            <span class="text-2xl animate-spin">🔄</span>
            Conectando ao GLPI e carregando lista de usuários...
        </div>
    `;

    const controller = new AbortController();
    const timeoutId = setTimeout(() => controller.abort(), 6000);

    try {
        const response = await fetch('/api/glpi/users', { signal: controller.signal });
        clearTimeout(timeoutId);

        if (!response.ok) {
            const errorMsg = await response.text();
            listContainer.innerHTML = `<div class="py-8 text-center text-rose-400 text-xs font-medium italic">⚠️ ${escapeHTML(errorMsg || "Erro ao conectar com a API do GLPI.")}</div>`;
            return;
        }

        allGLPIUsers = await response.json();
        filterModalGLPIUsers();
    } catch (err) {
        clearTimeout(timeoutId);
        let msg = err.name === 'AbortError' 
            ? 'A conexão com o GLPI demorou mais de 6 segundos.' 
            : (err.message || 'Falha de comunicação');
        listContainer.innerHTML = `<div class="py-8 text-center text-rose-400 text-xs font-medium italic">⚠️ ${escapeHTML(msg)}</div>`;
    } finally {
        if (btnSync) {
            btnSync.disabled = false;
            btnSync.innerText = '🔄 Sincronizar GLPI';
        }
    }
}

function filterModalGLPIUsers() {
    const searchInput = document.getElementById('modal-glpi-search');
    const counterEl = document.getElementById('modal-glpi-counter');
    if (!searchInput) return;

    const query = searchInput.value.toLowerCase().trim();
    let filtered = allGLPIUsers;
    if (query) {
        filtered = allGLPIUsers.filter(u => 
            (u.name && u.name.toLowerCase().includes(query)) || 
            (u.username && u.username.toLowerCase().includes(query))
        );
    }

    if (counterEl) {
        counterEl.innerText = `${filtered.length} usuários encontrados no GLPI`;
    }

    renderModalGLPIUsers(filtered);
}

let currentFilteredGLPIUsers = [];

function renderModalGLPIUsers(users) {
    const listContainer = document.getElementById('modal-glpi-user-list');
    if (!listContainer) return;

    currentFilteredGLPIUsers = users || [];

    if (!currentFilteredGLPIUsers || currentFilteredGLPIUsers.length === 0) {
        listContainer.innerHTML = `<div class="py-8 text-center text-zinc-500 text-xs italic">Nenhum usuário do GLPI encontrado com essa pesquisa.</div>`;
        return;
    }

    const localUsernames = new Set(systemUsersList.map(u => (u.username || '').toLowerCase()));

    let html = '';
    currentFilteredGLPIUsers.forEach((u, index) => {
        const isAlreadyInSystem = localUsernames.has((u.username || '').toLowerCase());

        html += `
            <div class="flex items-center justify-between p-3 bg-black/40 border border-white/5 rounded-2xl hover:border-white/10 transition">
                <div class="flex items-center gap-3">
                    <div class="w-8 h-8 rounded-full bg-zinc-800 border border-white/10 flex items-center justify-center text-zinc-300 font-bold text-xs uppercase">
                        ${escapeHTML((u.username || 'U').substring(0, 2))}
                    </div>
                    <div>
                        <div class="text-xs font-bold text-zinc-200">${escapeHTML(u.name || u.username)}</div>
                        <div class="text-[11px] font-mono text-zinc-400">@${escapeHTML(u.username)}</div>
                    </div>
                </div>

                <div class="flex items-center gap-2">
                    ${!isAlreadyInSystem ? `
                        <input type="password" id="modal-pass-${index}" placeholder="Senha (opcional)" class="bg-black/60 border border-white/10 text-xs p-1.5 rounded-xl text-zinc-200 focus:outline-none w-28">
                    ` : ''}
                    <select id="modal-role-${index}" class="bg-black/60 border border-white/10 text-xs p-1.5 rounded-xl text-zinc-200 focus:outline-none">
                        <option value="operator">👤 Operador</option>
                        <option value="admin">⭐ Administrador</option>
                    </select>

                    ${isAlreadyInSystem ? `
                        <span class="px-3 py-1.5 text-xs font-bold text-emerald-400 bg-emerald-500/10 border border-emerald-500/20 rounded-xl">
                            ✅ Importado
                        </span>
                    ` : `
                        <button onclick="importUserFromModal(${index})" class="px-3.5 py-1.5 text-xs font-bold text-black bg-gradient-to-r from-amber-400 to-amber-500 hover:from-amber-500 hover:to-amber-600 rounded-xl shadow-sm transition cursor-pointer">
                            ➕ Importar
                        </button>
                    `}
                </div>
            </div>
        `;
    });

    listContainer.innerHTML = html;
}

async function importUserFromModal(index) {
    const user = currentFilteredGLPIUsers[index];
    if (!user) return;

    const roleSelect = document.getElementById(`modal-role-${index}`);
    const selectedRole = roleSelect ? roleSelect.value : 'operator';
    const passInput = document.getElementById(`modal-pass-${index}`);
    const password = passInput ? passInput.value.trim() : '';

    const payload = {
        glpi_id: user.glpi_id || user.id || 0,
        username: user.username,
        name: user.name || user.username,
        password: password,
        role: selectedRole,
        enabled: true
    };

    try {
        const response = await fetch('/api/users/toggle-access', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify(payload)
        });

        if (!response.ok) {
            const errMsg = await response.text();
            throw new Error(errMsg || 'Erro ao importar usuário');
        }

        showToast(`Usuário @${user.username} importado com sucesso!`, '✅');
        
        await fetchSystemUsers();
        filterModalGLPIUsers();
    } catch (err) {
        showToast(err.message || 'Erro ao importar usuário.', '❌');
    }
}


