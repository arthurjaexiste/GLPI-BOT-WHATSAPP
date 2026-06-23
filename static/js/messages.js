let currentConfig = {};

// Carrega as configurações atuais da API
async function fetchConfig() {
    try {
        const response = await fetch('/api/config');
        if (!response.ok) throw new Error('Falha ao obter configurações');
        currentConfig = await response.json();
        
        document.getElementById('msg_novo_usuario').value = currentConfig.msg_novo_usuario || '';
        document.getElementById('msg_usuario_existente').value = currentConfig.msg_usuario_existente || '';
        document.getElementById('msg_menu_inicial').value = currentConfig.msg_menu_inicial || '';
        document.getElementById('msg_ticket_criado').value = currentConfig.msg_ticket_criado || '';
        document.getElementById('msg_fila_suporte').value = currentConfig.msg_fila_suporte || '';
        document.getElementById('msg_fila_espera').value = currentConfig.msg_fila_espera || '';
        document.getElementById('msg_suporte_assumido').value = currentConfig.msg_suporte_assumido || '';
        document.getElementById('msg_fim_atendimento').value = currentConfig.msg_fim_atendimento || '';
    } catch (err) {
        showToast('Erro ao obter as mensagens do sistema.', '❌');
    }
}

// Salva as configurações via POST na API
async function saveMessages() {
    currentConfig.msg_novo_usuario = document.getElementById('msg_novo_usuario').value;
    currentConfig.msg_usuario_existente = document.getElementById('msg_usuario_existente').value;
    currentConfig.msg_menu_inicial = document.getElementById('msg_menu_inicial').value;
    currentConfig.msg_ticket_criado = document.getElementById('msg_ticket_criado').value;
    currentConfig.msg_fila_suporte = document.getElementById('msg_fila_suporte').value;
    currentConfig.msg_fila_espera = document.getElementById('msg_fila_espera').value;
    currentConfig.msg_suporte_assumido = document.getElementById('msg_suporte_assumido').value;
    currentConfig.msg_fim_atendimento = document.getElementById('msg_fim_atendimento').value;

    try {
        const response = await fetch('/api/config', {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json'
            },
            body: JSON.stringify(currentConfig)
        });

        if (!response.ok) throw new Error('Erro ao salvar as mensagens');
        showToast('Mensagens de sistema atualizadas com sucesso!', '✅');
    } catch (err) {
        showToast('Erro ao salvar mensagens de sistema.', '❌');
    }
}

// Exibe um toast temporário
function showToast(message, icon = '✅') {
    const toast = document.getElementById('toast');
    const toastIcon = document.getElementById('toast-icon');
    const toastMsg = document.getElementById('toast-message');

    toastIcon.innerText = icon;
    toastMsg.innerText = message;

    if (icon === '✅') {
        toast.className = toast.className.replace('border-red-500/30', 'border-emerald-500/30')
                                       .replace('bg-red-950/80', 'bg-emerald-950/80')
                                       .replace('text-red-300', 'text-emerald-300') + ' border-emerald-500/30 bg-emerald-950/80 text-emerald-300';
    } else {
        toast.className = toast.className.replace('border-emerald-500/30', 'border-red-500/30')
                                       .replace('bg-emerald-950/80', 'bg-red-950/80')
                                       .replace('text-emerald-300', 'text-red-300') + ' border-red-500/30 bg-red-950/80 text-red-300';
    }

    toast.classList.remove('translate-y-24', 'opacity-0');
    
    setTimeout(() => {
        toast.classList.add('translate-y-24', 'opacity-0');
    }, 3000);
}

window.addEventListener('DOMContentLoaded', fetchConfig);
