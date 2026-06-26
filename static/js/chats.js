let activeChatJID = null;
let activeChatName = "";
let chatsData = [];

// Formata JID para exibir o número do telefone de forma legível
function formatJIDToPhone(jid) {
    if (!jid) return "";
    // Remove sufixos como @s.whatsapp.net ou grupos, e também session id :1, :2
    const num = jid.split('@')[0].split(':')[0];
    
    // Formatação de número brasileiro (+55 DD 9XXXX-XXXX ou +55 DD XXXX-XXXX)
    if (num.startsWith('55') && num.length >= 10) {
        const ddd = num.substring(2, 4);
        const rest = num.substring(4);
        if (rest.length === 9) {
            return `+55 (${ddd}) ${rest.substring(0, 5)}-${rest.substring(5)}`;
        } else if (rest.length === 8) {
            return `+55 (${ddd}) ${rest.substring(0, 4)}-${rest.substring(4)}`;
        }
    }
    return `+${num}`;
}

// Ao carregar a página
document.addEventListener("DOMContentLoaded", () => {
    loadChatsList();
    
    // Inicia polling periódico (atualiza mensagens a cada 3s e lista de chats a cada 6s)
    setInterval(() => {
        if (activeChatJID) {
            refreshActiveMessages();
        }
    }, 3000);

    setInterval(() => {
        loadChatsList(true); // silent load
    }, 6000);
});

// Busca a lista de chats e exibe na barra lateral
async function loadChatsList(silent = false) {
    try {
        const response = await fetch("/api/chats");
        if (!response.ok) throw new Error("Erro ao buscar conversas");
        
        chatsData = await response.json();
        renderChatsList();
    } catch (error) {
        console.error("Erro:", error);
        if (!silent) {
            const container = document.getElementById("chats-list-container");
            container.innerHTML = `<div class="text-center py-8 text-rose-400 text-xs italic">Erro ao carregar conversas.</div>`;
        }
    }
}

// Renderiza a lista de chats baseando-se no chatsData (e permite busca/filtro)
function renderChatsList(filteredData = null) {
    const container = document.getElementById("chats-list-container");
    const data = filteredData || chatsData;

    if (data.length === 0) {
        container.innerHTML = `<div class="text-center py-8 text-zinc-500 text-xs italic">Nenhuma conversa encontrada.</div>`;
        return;
    }

    container.innerHTML = "";
    data.forEach(chat => {
        // Status badge config
        let statusText = "Bot";
        let statusClass = "status-bot";
        if (chat.status === "live_chat") {
            statusText = "Suporte";
            statusClass = "status-live_chat";
        } else if (chat.status === "queue") {
            statusText = "Fila";
            statusClass = "status-queue";
        }

        const isSelected = activeChatJID === chat.jid;
        const cardClass = isSelected 
            ? "bg-white/10 border-white/10" 
            : "bg-zinc-900/40 hover:bg-white/5 border-white/5";

        const card = document.createElement("div");
        card.className = `p-3 rounded-xl border cursor-pointer transition flex flex-col gap-1.5 relative group ${cardClass}`;
        card.onclick = () => selectChat(chat.jid, chat.name, chat.status);

        // Limita o tamanho do texto da última mensagem
        let snippet = chat.last_message || "Nenhuma mensagem...";
        if (snippet.length > 35) snippet = snippet.substring(0, 35) + "...";

        const phoneFormatted = formatJIDToPhone(chat.jid);
        const displayName = chat.name ? chat.name : phoneFormatted;
        const subText = chat.name ? `<span class="text-[9px] text-zinc-400 font-mono">${phoneFormatted}</span>` : '';

        card.innerHTML = `
            <div class="flex items-center justify-between">
                <div class="flex items-center gap-2.5 min-w-0">
                    <img src="/api/chats/avatar?jid=${encodeURIComponent(chat.jid)}&name=${encodeURIComponent(displayName)}" class="w-8 h-8 rounded-lg object-cover border border-white/10 flex-shrink-0" onerror="this.onerror=null; this.src='https://ui-avatars.com/api/?name=${encodeURIComponent(displayName)}&background=random&color=fff';" />
                    <div class="flex flex-col min-w-0">
                        <span class="text-xs font-bold text-zinc-200 truncate max-w-[125px]">${displayName}</span>
                        ${subText}
                    </div>
                </div>
                <div class="flex items-center gap-1.5 flex-shrink-0">
                    <span class="status-dot ${statusClass} group-hover:hidden"></span>
                    <span class="text-[9px] font-bold text-zinc-500 uppercase tracking-widest group-hover:hidden">${statusText}</span>
                    <button onclick="event.stopPropagation(); deleteChat('${chat.jid}');" class="hidden group-hover:inline-flex items-center justify-center text-rose-400 hover:text-rose-300 transition text-xs p-1 hover:bg-white/5 rounded-md" title="Apagar conversa">
                        🗑️
                    </button>
                </div>
            </div>
            <div class="flex items-center justify-between gap-2 mt-1">
                <span class="text-[10px] text-zinc-400 truncate flex-1">${snippet}</span>
                <span class="text-[9px] text-zinc-500 font-medium">${chat.timestamp}</span>
            </div>
        `;
        container.appendChild(card);
    });
}

// Filtra os chats conforme digitação no search
function filterChats() {
    const term = document.getElementById("chat-search-input").value.toLowerCase();
    if (!term) {
        renderChatsList();
        return;
    }
    const filtered = chatsData.filter(chat => {
        return (chat.name && chat.name.toLowerCase().includes(term)) || 
               chat.jid.toLowerCase().includes(term) ||
               (chat.last_message && chat.last_message.toLowerCase().includes(term));
    });
    renderChatsList(filtered);
}

// Seleciona um chat da lista
async function selectChat(jid, name, status) {
    activeChatJID = jid;
    activeChatName = name || jid;
    
    // Esconde o placeholder
    document.getElementById("chat-placeholder").style.display = "none";
    
    // Configura o cabeçalho do chat ativo
    const phoneFormatted = formatJIDToPhone(jid);
    const displayName = name ? name : phoneFormatted;

    document.getElementById("active-chat-name").textContent = displayName;
    document.getElementById("active-chat-jid").textContent = phoneFormatted;
    
    // Configura o avatar no cabeçalho
    const avatarContainer = document.getElementById("active-chat-avatar");
    avatarContainer.innerHTML = `<img src="/api/chats/avatar?jid=${encodeURIComponent(jid)}&name=${encodeURIComponent(displayName)}" class="w-full h-full rounded-xl object-cover" onerror="this.onerror=null; this.src='https://ui-avatars.com/api/?name=${encodeURIComponent(displayName)}&background=random&color=fff';" />`;

    const statusDot = document.getElementById("active-chat-status-dot");
    const statusText = document.getElementById("active-chat-status-text");

    statusDot.className = "w-1.5 h-1.5 rounded-full";
    if (status === "live_chat") {
        statusDot.classList.add("status-live_chat");
        statusText.textContent = "Live Chat / Suporte";
    } else if (status === "queue") {
        statusDot.classList.add("status-queue");
        statusText.textContent = "Fila de Espera";
    } else {
        statusDot.classList.add("status-bot");
        statusText.textContent = "Interação Bot";
    }

    // Limpa a tela
    document.getElementById("chats-list-container").childNodes.forEach(node => {
        node.classList.remove("bg-white/10", "border-white/10");
        node.classList.add("bg-zinc-900/40", "border-white/5");
    });
    
    // Atualiza a lista lateral para destacar a selecionada
    loadChatsList(true); 

    // Carrega histórico de mensagens
    await refreshActiveMessages(true);
}

// Atualiza o histórico de mensagens
let lastMessagesCount = 0;
async function refreshActiveMessages(forceScroll = false) {
    if (!activeChatJID) return;
    
    try {
        const response = await fetch(`/api/chats/messages?jid=${encodeURIComponent(activeChatJID)}`);
        if (!response.ok) throw new Error("Erro ao buscar histórico");
        
        const messages = await response.json();
        renderMessages(messages, forceScroll);
    } catch (error) {
        console.error("Erro de sincronização de mensagens:", error);
    }
}

// Renderiza o histórico de mensagens
function renderMessages(messages, forceScroll = false) {
    const container = document.getElementById("chat-messages-container");
    
    // Se não há novas mensagens, evita re-renderizar para manter o scroll amigável do admin
    if (messages.length === lastMessagesCount && !forceScroll) {
        return;
    }
    
    container.innerHTML = "";
    lastMessagesCount = messages.length;

    if (messages.length === 0) {
        container.innerHTML = `<div class="text-center py-8 text-zinc-500 text-xs italic">Nenhuma mensagem nesta conversa.</div>`;
        return;
    }

    messages.forEach(msg => {
        const row = document.createElement("div");
        row.className = `w-full flex ${msg.is_from_me ? 'justify-end' : 'justify-start'}`;

        const bubble = document.createElement("div");
        bubble.className = `message-bubble ${msg.is_from_me ? 'message-outgoing' : 'message-incoming'}`;
        
        const formattedText = msg.text.replace(/\n/g, "<br>");
        
        bubble.innerHTML = `
            <div class="font-medium">${formattedText}</div>
            <div class="text-[9px] mt-1 text-right ${msg.is_from_me ? 'text-zinc-600' : 'text-zinc-400'}">${msg.timestamp}</div>
        `;
        row.appendChild(bubble);
        container.appendChild(row);
    });

    // Rola para a base das mensagens
    container.scrollTop = container.scrollHeight;
}

// Envia mensagem via Painel Console
async function sendConsoleMessage() {
    const input = document.getElementById("chat-message-input");
    const text = input.value.trim();
    
    if (!text || !activeChatJID) return;
    
    const sendBtn = document.getElementById("btn-send-msg");
    sendBtn.disabled = true;
    sendBtn.textContent = "Enviando...";

    try {
        const response = await fetch("/api/chats/send", {
            method: "POST",
            headers: {
                "Content-Type": "application/json"
            },
            body: JSON.stringify({
                jid: activeChatJID,
                text: text
            })
        });

        if (!response.ok) throw new Error("Falha ao enviar");
        
        input.value = "";
        showToast("Mensagem enviada com sucesso!");
        await refreshActiveMessages(true);
        loadChatsList(true); // Atualiza snippet lateral
    } catch (error) {
        console.error(error);
        showToast("Falha ao enviar mensagem.", false);
    } finally {
        sendBtn.disabled = false;
        sendBtn.textContent = "Enviar";
    }
}

// Detecta Enter no input
function handleInputKey(e) {
    if (e.key === "Enter") {
        sendConsoleMessage();
    }
}

// Exibe Toast de notificação
function showToast(message, isSuccess = true) {
    const toast = document.getElementById("toast");
    const icon = document.getElementById("toast-icon");
    const msg = document.getElementById("toast-message");

    icon.textContent = isSuccess ? "✅" : "🚨";
    msg.textContent = message;
    
    toast.className = `fixed bottom-6 right-6 px-5 py-3 rounded-lg shadow-2xl backdrop-blur-md transform transition duration-300 flex items-center gap-3 z-50 ${
        isSuccess 
        ? 'bg-emerald-950/80 border border-emerald-500/30 text-emerald-300' 
        : 'bg-rose-950/80 border border-rose-500/30 text-rose-300'
    }`;

    toast.classList.remove("translate-y-24", "opacity-0");
    
    setTimeout(() => {
        toast.classList.add("translate-y-24", "opacity-0");
    }, 3000);
}

// Apaga uma conversa e limpa o histórico
async function deleteChat(jid) {
    if (!confirm("Tem certeza que deseja apagar esta conversa e todo o seu histórico? Esta ação é irreversível e resetará o atendimento do bot para este contato.")) {
        return;
    }

    try {
        const response = await fetch(`/api/chats/delete?jid=${encodeURIComponent(jid)}`, {
            method: "DELETE"
        });

        if (!response.ok) throw new Error("Erro ao apagar conversa");

        showToast("Conversa apagada com sucesso!");

        // Se a conversa apagada for a atualmente ativa, limpa a janela de chat e mostra placeholder
        if (activeChatJID === jid) {
            activeChatJID = null;
            activeChatName = "";
            document.getElementById("chat-placeholder").style.display = "flex";
        }

        // Recarrega a lista de chats
        loadChatsList();
    } catch (error) {
        console.error(error);
        showToast("Falha ao apagar conversa.", false);
    }
}

// Apaga a conversa ativa atualmente
async function deleteActiveChat() {
    if (!activeChatJID) return;
    await deleteChat(activeChatJID);
}

// Finaliza o atendimento ativo atualmente
async function closeActiveChat() {
    if (!activeChatJID) return;
    await closeChat(activeChatJID);
}

// Finaliza o atendimento (devolve para o Bot)
async function closeChat(jid) {
    if (!confirm("Deseja finalizar o atendimento humano e reativar o Bot para esta conversa?")) {
        return;
    }

    try {
        const response = await fetch(`/api/chats/close?jid=${encodeURIComponent(jid)}`, {
            method: "POST"
        });

        if (!response.ok) throw new Error("Erro ao finalizar atendimento");

        showToast("Atendimento finalizado. Bot reativado!");

        // Recarrega a lista de chats para atualizar o status visual
        loadChatsList();
        
        // Atualiza cabeçalho do chat ativo para mostrar "Interação Bot"
        const statusDot = document.getElementById("active-chat-status-dot");
        const statusText = document.getElementById("active-chat-status-text");
        if (statusDot && statusText) {
            statusDot.className = "w-1.5 h-1.5 rounded-full status-bot";
            statusText.textContent = "Interação Bot";
        }
    } catch (error) {
        console.error(error);
        showToast("Falha ao finalizar atendimento.", false);
    }
}
