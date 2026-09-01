let currentConfig = {};

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

        document.getElementById('msg_enquete_documentos').value = currentConfig.msg_enquete_documentos || '';
        document.getElementById('msg_enviar_documentos').value = currentConfig.msg_enviar_documentos || '';
        document.getElementById('msg_enquete_confirmar_documentos').value = currentConfig.msg_enquete_confirmar_documentos || '';
        document.getElementById('msg_documento_adicionado').value = currentConfig.msg_documento_adicionado || '';
        document.getElementById('msg_proximo_documento').value = currentConfig.msg_proximo_documento || '';
        document.getElementById('msg_enquete_fotos').value = currentConfig.msg_enquete_fotos || '';
        document.getElementById('msg_enquete_fotos_pos_docs').value = currentConfig.msg_enquete_fotos_pos_docs || '';
        document.getElementById('msg_enviar_fotos').value = currentConfig.msg_enviar_fotos || '';
        document.getElementById('msg_enquete_confirmar_fotos').value = currentConfig.msg_enquete_confirmar_fotos || '';
        document.getElementById('msg_foto_adicionada').value = currentConfig.msg_foto_adicionada || '';
        document.getElementById('msg_proxima_foto').value = currentConfig.msg_proxima_foto || '';
    } catch (err) {
        showToast('Erro ao obter as mensagens do sistema.', false);
    }
}

async function saveMessages() {
    currentConfig.msg_novo_usuario = document.getElementById('msg_novo_usuario').value;
    currentConfig.msg_usuario_existente = document.getElementById('msg_usuario_existente').value;
    currentConfig.msg_menu_inicial = document.getElementById('msg_menu_inicial').value;
    currentConfig.msg_ticket_criado = document.getElementById('msg_ticket_criado').value;
    currentConfig.msg_fila_suporte = document.getElementById('msg_fila_suporte').value;
    currentConfig.msg_fila_espera = document.getElementById('msg_fila_espera').value;
    currentConfig.msg_suporte_assumido = document.getElementById('msg_suporte_assumido').value;
    currentConfig.msg_fim_atendimento = document.getElementById('msg_fim_atendimento').value;

    currentConfig.msg_enquete_documentos = document.getElementById('msg_enquete_documentos').value;
    currentConfig.msg_enviar_documentos = document.getElementById('msg_enviar_documentos').value;
    currentConfig.msg_enquete_confirmar_documentos = document.getElementById('msg_enquete_confirmar_documentos').value;
    currentConfig.msg_documento_adicionado = document.getElementById('msg_documento_adicionado').value;
    currentConfig.msg_proximo_documento = document.getElementById('msg_proximo_documento').value;
    currentConfig.msg_enquete_fotos = document.getElementById('msg_enquete_fotos').value;
    currentConfig.msg_enquete_fotos_pos_docs = document.getElementById('msg_enquete_fotos_pos_docs').value;
    currentConfig.msg_enviar_fotos = document.getElementById('msg_enviar_fotos').value;
    currentConfig.msg_enquete_confirmar_fotos = document.getElementById('msg_enquete_confirmar_fotos').value;
    currentConfig.msg_foto_adicionada = document.getElementById('msg_foto_adicionada').value;
    currentConfig.msg_proxima_foto = document.getElementById('msg_proxima_foto').value;

    try {
        const response = await fetch('/api/config', {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json'
            },
            body: JSON.stringify(currentConfig)
        });

        if (!response.ok) throw new Error('Erro ao salvar as mensagens');
        showToast('Mensagens de sistema atualizadas com sucesso!', true);
    } catch (err) {
        showToast('Erro ao salvar mensagens de sistema.', false);
    }
}

function showToast(message, isSuccess = true) {
    const toast = document.getElementById('toast');
    const toastIcon = document.getElementById('toast-icon');
    const toastMsg = document.getElementById('toast-message');

    if (toastIcon) toastIcon.innerText = '';
    if (toastMsg) toastMsg.innerText = message;

    if (isSuccess) {
        toast.className = "fixed bottom-4 right-4 px-4 py-2 rounded bg-slate-900 border border-slate-700 text-emerald-400 font-semibold transition duration-200 flex items-center gap-2 z-50 text-xs";
    } else {
        toast.className = "fixed bottom-4 right-4 px-4 py-2 rounded bg-slate-900 border border-slate-700 text-rose-400 font-semibold transition duration-200 flex items-center gap-2 z-50 text-xs";
    }

    toast.classList.remove('translate-y-24', 'opacity-0');

    setTimeout(() => {
        toast.classList.add('translate-y-24', 'opacity-0');
    }, 3000);
}

document.addEventListener('DOMContentLoaded', fetchConfig);
