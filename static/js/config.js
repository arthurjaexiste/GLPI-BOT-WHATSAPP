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

let initialConfigState = null;

function getFormState() {
    const inputs = document.querySelectorAll('input, select, textarea');
    const state = {};
    inputs.forEach(el => {
        if (!el.id || el.id === 'system-user-search' || el.id === 'modal-glpi-search' || el.id === 'modal-new-password') return;
        if (el.type === 'checkbox') {
            state[el.id] = el.checked;
        } else {
            state[el.id] = el.value.trim();
        }
    });

    if (typeof systemUsersList !== 'undefined' && systemUsersList) {
        state['__users__'] = systemUsersList.map(u => `${u.id}:${u.enabled}:${u.role}`).join('|');
    }
    return JSON.stringify(state);
}

function checkConfigDirtyState() {
    if (!initialConfigState) return;
    const currentState = getFormState();
    if (currentState === initialConfigState) {
        markConfigClean();
    } else {
        markConfigDirty();
    }
}

function markConfigDirty() {
    const btn = document.getElementById('btn-header-save');
    if (!btn) return;
    btn.disabled = false;
    btn.className = "px-4 py-2 text-xs font-medium text-amber-400 bg-amber-500/10 hover:bg-amber-500/20 rounded-lg ring-1 ring-amber-500/20 transition-all flex items-center gap-2 cursor-pointer";
    btn.innerHTML = `<svg class="w-3.5 h-3.5" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M8 7H5a2 2 0 00-2 2v9a2 2 0 002 2h14a2 2 0 002-2V9a2 2 0 00-2-2h-3m-1 4l-3 3m0 0l-3-3m3 3V4"/></svg><span>Salvar Configurações</span>`;
}

function markConfigClean() {
    const btn = document.getElementById('btn-header-save');
    if (!btn) return;
    btn.disabled = true;
    btn.className = "px-4 py-2 text-xs font-medium rounded-lg transition-all flex items-center gap-2 opacity-40 pointer-events-none bg-white/[0.03] text-neutral-500 ring-1 ring-white/[0.05]";
    btn.innerHTML = `<svg class="w-3.5 h-3.5 text-emerald-400" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M5 13l4 4L19 7"/></svg><span>Salvo</span>`;
}

// Event Delegation Global para captura de qualquer alteração nos formulários
document.addEventListener('input', (e) => {
    if (!e.target || e.target.id === 'system-user-search' || e.target.id === 'modal-glpi-search' || e.target.id === 'modal-new-password') return;
    checkConfigDirtyState();
});

document.addEventListener('change', (e) => {
    if (!e.target || e.target.id === 'system-user-search' || e.target.id === 'modal-glpi-search' || e.target.id === 'modal-new-password') return;
    checkConfigDirtyState();
});

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

        // Atualiza estado ativado/desativado dos campos de SMTP e Expediente
        toggleSMTPFields();
        toggleExpedienteFields();

        const smtpToggle = document.getElementById('smtp_enabled');
        if (smtpToggle) smtpToggle.onchange = toggleSMTPFields;

        const expToggle = document.getElementById('working_hours_enabled');
        if (expToggle) expToggle.onchange = toggleExpedienteFields;

        initialConfigState = getFormState();
        markConfigClean();
        updateLivePreview();
    } catch (err) {
        showToast('Erro ao obter as configurações.', '❌');
    }
}

function toggleSMTPFields() {
    const toggle = document.getElementById('smtp_enabled');
    if (!toggle) return;
    const isChecked = toggle.checked;
    const smtpInputs = ['smtp_host', 'smtp_port', 'smtp_username', 'smtp_password', 'smtp_sender', 'smtp_receiver'];
    const badge = document.getElementById('smtp-status-badge');

    if (badge) {
        badge.innerHTML = isChecked 
            ? `<span class="inline-flex items-center gap-1.5 px-2.5 py-1 rounded-full text-[10px] font-bold bg-amber-500/10 text-amber-400 border border-amber-500/20"><span class="w-1.5 h-1.5 rounded-full bg-amber-400 animate-pulse"></span> Alertas Ativos</span>`
            : `<span class="inline-flex items-center gap-1.5 px-2.5 py-1 rounded-full text-[10px] font-bold bg-zinc-800/80 text-zinc-500 border border-zinc-700/50">Desativado</span>`;
    }

    smtpInputs.forEach(id => {
        const el = document.getElementById(id);
        if (!el) return;
        el.disabled = !isChecked;
        if (!isChecked) {
            el.classList.add('opacity-40', 'cursor-not-allowed', 'bg-zinc-950/80');
            el.classList.remove('bg-zinc-900');
        } else {
            el.classList.remove('opacity-40', 'cursor-not-allowed', 'bg-zinc-950/80');
            el.classList.add('bg-zinc-900');
        }
    });
}

function toggleExpedienteFields() {
    const toggle = document.getElementById('working_hours_enabled');
    if (!toggle) return;
    const isChecked = toggle.checked;
    const expInputs = ['working_hours_start', 'working_hours_end', 'msg_ausencia'];
    const badge = document.getElementById('expediente-status-badge');

    if (badge) {
        badge.innerHTML = isChecked 
            ? `<span class="inline-flex items-center gap-1.5 px-2.5 py-1 rounded-full text-[10px] font-bold bg-amber-500/10 text-amber-400 border border-amber-500/20"><span class="w-1.5 h-1.5 rounded-full bg-amber-400 animate-pulse"></span> Controle Ativo</span>`
            : `<span class="inline-flex items-center gap-1.5 px-2.5 py-1 rounded-full text-[10px] font-bold bg-zinc-800/80 text-zinc-500 border border-zinc-700/50">Desativado</span>`;
    }

    expInputs.forEach(id => {
        const el = document.getElementById(id);
        if (!el) return;
        el.disabled = !isChecked;
        if (!isChecked) {
            el.classList.add('opacity-40', 'cursor-not-allowed', 'bg-zinc-950/80');
            el.classList.remove('bg-zinc-900');
        } else {
            el.classList.remove('opacity-40', 'cursor-not-allowed', 'bg-zinc-950/80');
            el.classList.add('bg-zinc-900');
        }
    });
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

        if (typeof systemUsersList !== 'undefined' && systemUsersList && systemUsersList.length > 0) {
            for (let i = 0; i < systemUsersList.length; i++) {
                await saveSystemUserDirect(i, true);
            }
        }

        showToast('Configurações salvas e aplicadas com sucesso!', '✅');
        initialConfigState = getFormState();
        markConfigClean();
    } catch (err) {
        showToast('Erro ao salvar configurações.', '❌');
    }
}

// Alterna entre mostrar e esconder a senha (tipo password / text)

// Função toggleVisibility manipula a rotina correspondente na interface do painel
function toggleVisibility(btn, id) {
    const input = document.getElementById(id);
    if (!input) return;
    const isPassword = input.type === "password";
    input.type = isPassword ? "text" : "password";
    btn.innerHTML = isPassword 
        ? `<svg class="w-4 h-4 text-amber-400" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M13.875 18.825A10.05 10.05 0 0112 19c-4.478 0-8.268-2.943-9.543-7a9.97 9.97 0 011.563-3.029m5.858-5.908a10.047 10.047 0 013.682-.763c4.478 0 8.268 2.943 9.542 7a10.025 10.025 0 01-4.132 5.411m-4.276-4.276a3 3 0 10-4.243-4.243M3 3l18 18"/></svg>`
        : `<svg class="w-4 h-4 text-zinc-400 hover:text-zinc-200" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M15 12a3 3 0 11-6 0 3 3 0 016 0z"/><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M2.458 12C3.732 7.943 7.523 5 12 5c4.478 0 8.268 2.943 9.542 7-1.274 4.057-5.064 7-9.542 7-4.477 0-8.268-2.943-9.542-7z"/></svg>`;
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

function updateLivePreview() {
    const compName = document.getElementById('company_name')?.value.trim() || 'Sua Empresa';
    const adminPhone = document.getElementById('telefone_notificacao')?.value.trim() || 'Não configurado';

    const pCompName = document.getElementById('preview-company-name');
    const pCompBold = document.getElementById('preview-company-bold');
    const pAdminPhone = document.getElementById('preview-admin-phone');

    if (pCompName) pCompName.innerText = compName + ' Bot';
    if (pCompBold) pCompBold.innerText = compName;
    if (pAdminPhone) pAdminPhone.innerText = adminPhone;
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
        t.className = "tab-btn px-3.5 py-2.5 text-left text-xs font-normal rounded-lg transition-colors flex items-center gap-3 whitespace-nowrap text-neutral-400 hover:text-white hover:bg-white/5 cursor-pointer";
        const svg = t.querySelector('svg');
        if (svg) svg.className = "w-4 h-4 text-neutral-400 shrink-0";
    });

    // Aplica estilo ativo na aba selecionada
    const activeTab = document.getElementById('tab-' + tabName);
    if (activeTab) {
        activeTab.className = "tab-btn px-3.5 py-2.5 text-left text-xs font-medium rounded-lg transition-colors flex items-center gap-3 whitespace-nowrap text-white bg-white/10 ring-1 ring-white/[0.08] active cursor-pointer";
        const svg = activeTab.querySelector('svg');
        if (svg) svg.className = "w-4 h-4 text-amber-400 shrink-0";
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
        initialConfigState = getFormState();
        checkConfigDirtyState();
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
        tbody.innerHTML = `<tr><td colspan="5" class="py-6 text-center text-neutral-500 italic font-normal">Nenhum usuário cadastrado no sistema ainda. Clique em "Importar Usuário do GLPI" acima.</td></tr>`;
        return;
    }

    let html = '';
    users.forEach((u, index) => {
        const isChecked = u.enabled ? 'checked' : '';
        const statusBadge = u.enabled 
            ? `<span class="inline-flex items-center gap-1.5 px-2.5 py-0.5 rounded-full text-xs font-medium bg-emerald-500/10 text-emerald-400 ring-1 ring-emerald-500/20"><span class="w-1.5 h-1.5 rounded-full bg-emerald-400 animate-pulse"></span> Ativo</span>`
            : `<span class="inline-flex items-center gap-1.5 px-2.5 py-0.5 rounded-full text-xs font-medium bg-white/5 text-neutral-400 ring-1 ring-white/10"><span class="w-1.5 h-1.5 rounded-full bg-neutral-500"></span> Inativo</span>`;

        const roleSelect = `
            <select id="user-role-select-${index}" onchange="updateSystemUserPermissions(${index})" class="w-32 bg-white/5 text-xs text-white/90 px-2.5 py-1 rounded-lg ring-1 ring-white/10 focus:ring-white/20 focus:outline-none cursor-pointer">
                <option value="operator" ${u.role === 'operator' ? 'selected' : ''}>Operador</option>
                <option value="admin" ${u.role === 'admin' ? 'selected' : ''}>Administrador</option>
            </select>
        `;

        html += `
            <tr class="border-b border-white/[0.04] hover:bg-white/[0.02] transition-colors">
                <td class="py-3 px-3 align-middle font-medium text-neutral-200">${escapeHTML(u.name || u.username)}</td>
                <td class="py-3 px-3 align-middle font-mono text-xs text-neutral-400">@${escapeHTML(u.username)}</td>
                <td class="py-3 px-3 align-middle">
                    <div class="flex items-center gap-3">
                        <label class="relative inline-flex items-center cursor-pointer">
                            <input type="checkbox" id="user-toggle-${index}" ${isChecked} onchange="updateSystemUserPermissions(${index})" class="sr-only peer">
                            <div class="w-8 h-4.5 bg-neutral-800 peer-focus:outline-none rounded-full peer peer-checked:after:translate-x-full peer-checked:after:border-white after:content-[''] after:absolute after:top-[2px] after:left-[2px] after:bg-neutral-400 after:rounded-full after:h-3.5 after:w-3.5 after:transition-all peer-checked:bg-amber-500"></div>
                        </label>
                        ${statusBadge}
                    </div>
                </td>
                <td class="py-3 px-3 align-middle">${roleSelect}</td>
                <td class="py-3 px-3 align-middle text-right flex items-center justify-end gap-1.5">
                    <button onclick="openSetPasswordModal('${escapeHTML(u.username)}', ${u.id}, '${escapeHTML(u.name || u.username)}')" class="px-2.5 py-1 text-xs font-medium text-neutral-400 hover:text-white bg-transparent hover:bg-white/5 rounded-lg transition-colors flex items-center gap-1.5 cursor-pointer" title="Definir Senha">
                        <svg class="w-3.5 h-3.5 text-neutral-400 hover:text-amber-400" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M15 7a2 2 0 012 2m4 0a6 6 0 01-7.743 5.743L11 17H9v2H7v2H4a1 1 0 01-1-1v-2.586a1 1 0 01.293-.707l5.964-5.964A6 6 0 1121 9z"/></svg>
                        <span>Senha</span>
                    </button>
                    <button onclick="deleteSystemUser('${escapeHTML(u.username)}')" class="p-1.5 text-neutral-500 hover:text-red-400 bg-transparent hover:bg-red-400/10 rounded-lg transition-colors flex items-center justify-center cursor-pointer" title="Remover Usuário">
                        <svg class="w-3.5 h-3.5" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16"/></svg>
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

function updateSystemUserPermissions(index) {
    const user = systemUsersList[index];
    if (!user) return;

    const toggle = document.getElementById(`user-toggle-${index}`);
    const roleSelect = document.getElementById(`user-role-select-${index}`);

    if (toggle) user.enabled = toggle.checked;
    if (roleSelect) user.role = roleSelect.value;

    renderSystemUsersTable(systemUsersList);
    markConfigDirty();
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

function getModalGLPIListContainer() {
    return document.getElementById('modal-glpi-user-list') || document.getElementById('modal-glpi-users-list');
}

async function syncModalGLPIUsers() {
    const listContainer = getModalGLPIListContainer();
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
    const listContainer = getModalGLPIListContainer();
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
            <div class="flex items-center justify-between gap-4 p-3.5 bg-zinc-900/60 border border-white/5 rounded-2xl hover:border-white/10 transition">
                <div class="flex items-center gap-3 min-w-0">
                    <div class="w-9 h-9 rounded-full bg-zinc-800 border border-white/10 flex items-center justify-center text-zinc-300 font-bold text-xs uppercase shrink-0">
                        ${escapeHTML((u.username || 'U').substring(0, 2))}
                    </div>
                    <div class="truncate">
                        <div class="text-xs font-semibold text-zinc-100 truncate">${escapeHTML(u.name || u.username)}</div>
                        <div class="text-[11px] font-mono text-zinc-400 truncate">@${escapeHTML(u.username)}</div>
                    </div>
                </div>

                <div class="flex items-center gap-3 shrink-0">
                    <select id="modal-role-${index}" class="h-9 w-36 bg-zinc-900 border border-white/10 text-xs px-3 rounded-xl text-zinc-200 focus:border-amber-500/50 focus:outline-none cursor-pointer">
                        <option value="operator" ${u.role === 'operator' ? 'selected' : ''}>Operador</option>
                        <option value="admin" ${u.role === 'admin' ? 'selected' : ''}>Administrador</option>
                    </select>

                    ${isAlreadyInSystem ? `
                        <span class="h-9 px-4 inline-flex items-center gap-1.5 text-xs font-medium text-emerald-400 bg-emerald-500/10 border border-emerald-500/20 rounded-xl">
                            <svg class="w-3.5 h-3.5" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M5 13l4 4L19 7"/></svg>
                            <span>Importado</span>
                        </span>
                    ` : `
                        <button onclick="importUserFromModal(${index})" class="h-9 px-4 inline-flex items-center justify-center gap-1.5 text-xs font-semibold text-black bg-gradient-to-r from-amber-400 to-yellow-500 hover:from-amber-300 hover:to-amber-400 rounded-xl shadow-md hover:shadow-amber-500/20 transition-all duration-200 cursor-pointer border border-amber-300/30">
                            <svg class="w-3.5 h-3.5" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 4v16m8-8H4"/></svg>
                            <span>Importar</span>
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

    const payload = {
        glpi_id: user.glpi_id || user.id || 0,
        username: user.username,
        name: user.name || user.username,
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




