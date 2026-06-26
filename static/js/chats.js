let activeChatJID = null;
let activeChatName = "";
let chatsData = [];

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
        card.className = `p-3 rounded-xl border cursor-pointer transition flex flex-col gap-1.5 ${cardClass}`;
        card.onclick = () => selectChat(chat.jid, chat.name, chat.status);

        // Limita o tamanho do texto da última mensagem
        let snippet = chat.last_message || "Nenhuma mensagem...";
        if (snippet.length > 35) snippet = snippet.substring(0, 35) + "...";

        // Extrai o primeiro caractere para o avatar
        const initial = chat.name ? chat.name.charAt(0).toUpperCase() : "?";

        card.innerHTML = `
            <div class="flex items-center justify-between">
                <div class="flex items-center gap-2">
                    <div class="w-7 h-7 rounded-lg bg-white/5 border border-white/10 flex items-center justify-center text-xs font-bold text-zinc-300">${initial}</div>
                    <span class="text-xs font-bold text-zinc-200 truncate max-w-[120px]">${chat.name || chat.jid}</span>
                </div>
                <div class="flex items-center gap-1.5">
                    <span class="status-dot ${statusClass}"></span>
                    <span class="text-[9px] font-bold text-zinc-500 uppercase tracking-widest">${statusText}</span>
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
    document.getElementById("active-chat-name").textContent = activeChatName;
    document.getElementById("active-chat-jid").textContent = jid;
    document.getElementById("active-chat-avatar").textContent = activeChatName.charAt(0).toUpperCase();

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
