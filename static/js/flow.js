let flowRoot = null;
let activeEditPath = []; // Caminho de índices para localizar o nó na árvore
let activeEditMode = "edit"; // "edit" ou "add"

// Carrega o fluxo de conversa via API
async function fetchFlow() {
    try {
        const response = await fetch('/api/flow');
        if (!response.ok) throw new Error('Erro ao obter fluxo');
        flowRoot = await response.json();
        renderTree();
    } catch (error) {
        console.error(error);
        showToast('Erro ao obter o fluxo.', '❌');
    }
}

// Renderiza a árvore recursivamente
function renderTree() {
    const treeContainer = document.getElementById('flow-tree');
    treeContainer.innerHTML = '';

    if (!flowRoot) return;

    // Renderizar a raiz (que é o Menu Inicial, não deletável)
    const rootEl = createNodeUI(flowRoot, [], true);
    treeContainer.appendChild(rootEl);
}

// Gera o HTML do nó
function createNodeUI(node, path, isRoot = false) {
    const div = document.createElement('div');
    div.className = `flex flex-col gap-3 rounded-2xl p-5 transition duration-200 ${isRoot ? 'bg-indigo-950/20 border border-indigo-500/20 shadow-lg shadow-indigo-500/5' : 'bg-zinc-900/40 border border-white/5 ml-6 hover:border-white/10'}`;

    // Emblemas de acordo com o tipo
    let badgeHTML = '';
    switch (node.type) {
        case 'menu':
            const backInfo = (isRoot || node.show_back_button !== false) ? ' (+Botão Voltar)' : ' (Sem Voltar)';
            badgeHTML = `<span class="text-[10px] font-bold bg-indigo-500/20 text-indigo-300 border border-indigo-500/20 px-2 py-0.5 rounded">📁 Menu${backInfo}</span>`;
            break;
        case 'ticket':
            const attachmentTypes = [];
            if (node.ask_images !== false) attachmentTypes.push('Imagens');
            if (node.ask_docs !== false) attachmentTypes.push('Documentos');
            const attachmentsInfo = attachmentTypes.length > 0 ? ` (+Anexos: ${attachmentTypes.join('/')})` : ' (Sem Anexos)';
            badgeHTML = `<span class="text-[10px] font-bold bg-emerald-500/20 text-emerald-300 border border-emerald-500/20 px-2 py-0.5 rounded">🎫 Chamado GLPI (ID: ${node.glpi_id || 0})${attachmentsInfo}</span>`;
            break;
        case 'text':
            badgeHTML = '<span class="text-[10px] font-bold bg-cyan-500/20 text-cyan-300 border border-cyan-500/20 px-2 py-0.5 rounded">💬 Resposta / FAQ</span>';
            break;
        case 'status':
            badgeHTML = '<span class="text-[10px] font-bold bg-purple-500/20 text-purple-300 border border-purple-500/20 px-2 py-0.5 rounded">🔍 Status Chamado</span>';
            break;
        case 'human':
            badgeHTML = '<span class="text-[10px] font-bold bg-amber-500/20 text-amber-300 border border-amber-500/20 px-2 py-0.5 rounded">👤 Falar c/ Suporte</span>';
            break;
    }

    // Ações de ordenação
    const pathStr = JSON.stringify(path);
    const parentPath = path.slice(0, -1);
    const childIdx = path[path.length - 1];

    const sortButtons = isRoot ? '' : `
        <button onclick="moveNode(${JSON.stringify(parentPath)}, ${childIdx}, -1)" class="p-1 hover:bg-gray-700 rounded text-gray-400 hover:text-white text-xs">▲</button>
        <button onclick="moveNode(${JSON.stringify(parentPath)}, ${childIdx}, 1)" class="p-1 hover:bg-gray-700 rounded text-gray-400 hover:text-white text-xs">▼</button>
    `;

    // Botão de adicionar filho (só para Menu)
    const addChildButton = node.type === 'menu' ? `
        <button onclick="openAddModal(${pathStr})" class="px-2.5 py-1 text-[11px] font-bold bg-violet-600/20 text-violet-300 hover:bg-violet-600/40 rounded border border-violet-500/20 transition duration-150">
            + Adicionar Opção
        </button>
    ` : '';

    // Botão de excluir
    const deleteButton = isRoot ? '' : `
        <button onclick="deleteNode(${JSON.stringify(parentPath)}, ${childIdx})" class="p-1 hover:bg-red-950/60 hover:text-red-400 rounded text-gray-500 text-xs ml-1">🗑️</button>
    `;

    div.innerHTML = `
        <div class="flex items-center justify-between gap-4">
            <div class="flex items-center gap-3">
                <span class="text-sm font-semibold">${node.title}</span>
                ${badgeHTML}
            </div>
            <div class="flex items-center gap-2">
                ${addChildButton}
                ${sortButtons}
                <button onclick="openEditModal(${pathStr})" class="p-1 hover:bg-gray-700 rounded text-gray-400 hover:text-white text-xs">✏️</button>
                ${deleteButton}
            </div>
        </div>
    `;

    // Se for menu e tiver filhos, renderizar filhos recursivamente
    if (node.type === 'menu' && node.children && node.children.length > 0) {
        const childrenDiv = document.createElement('div');
        childrenDiv.className = 'flex flex-col gap-2 border-l border-gray-800 ml-2 mt-1';
        
        node.children.forEach((child, idx) => {
            const childEl = createNodeUI(child, [...path, idx]);
            childrenDiv.appendChild(childEl);
        });
        
        div.appendChild(childrenDiv);
    } else if (node.type === 'menu') {
        const emptyDiv = document.createElement('div');
        emptyDiv.className = 'text-xs text-gray-500 italic ml-6 py-2 border-l border-gray-800 pl-4 border-dashed';
        emptyDiv.innerText = 'Menu vazio. Adicione opções filhas.';
        div.appendChild(emptyDiv);
    }

    return div;
}

// Localiza um nó na árvore pelo caminho de índices
function funcNodeByPath(path) {
    let cur = flowRoot;
    for (let idx of path) {
        cur = cur.children[idx];
    }
    return cur;
}

// Abre o modal de edição
function openEditModal(path) {
    activeEditPath = path;
    activeEditMode = "edit";
    const node = funcNodeByPath(path);

    document.getElementById('modal-title').innerText = "Editar Opção";
    document.getElementById('node-title').value = node.title || '';
    document.getElementById('node-type').value = node.type || 'menu';
    document.getElementById('node-glpi').value = node.glpi_id || 0;
    document.getElementById('node-content').value = node.content || '';
    
    // Configura os checkboxes (padrão é true para nós existentes)
    document.getElementById('node-ask-images').checked = node.ask_images !== false;
    document.getElementById('node-ask-docs').checked = node.ask_docs !== false;
    document.getElementById('node-show-back').checked = node.show_back_button !== false;

    // Raiz não pode ter tipo alterado (sempre menu)
    document.getElementById('node-type').disabled = path.length === 0;

    toggleModalFields();
    document.getElementById('edit-modal').classList.remove('hidden');
}

// Abre o modal para adicionar
function openAddModal(path) {
    activeEditPath = path;
    activeEditMode = "add";

    document.getElementById('modal-title').innerText = "Adicionar Nova Opção";
    document.getElementById('node-title').value = '';
    document.getElementById('node-type').value = 'ticket';
    document.getElementById('node-glpi').value = 0;
    document.getElementById('node-content').value = '';
    
    // Checkboxes marcados por padrão ao adicionar novo nó
    document.getElementById('node-ask-images').checked = true;
    document.getElementById('node-ask-docs').checked = true;
    document.getElementById('node-show-back').checked = true;

    document.getElementById('node-type').disabled = false;

    toggleModalFields();
    document.getElementById('edit-modal').classList.remove('hidden');
}

function closeModal() {
    document.getElementById('edit-modal').classList.add('hidden');
}

// Esconde ou mostra os campos de acordo com o tipo
function toggleModalFields() {
    const type = document.getElementById('node-type').value;
    const fieldGLPI = document.getElementById('field-glpi');
    const fieldContent = document.getElementById('field-content');
    const labelContent = document.getElementById('label-content');
    const fieldAttachments = document.getElementById('field-attachments');
    const fieldMenuOptions = document.getElementById('field-menu-options');

    fieldGLPI.classList.add('hidden');
    fieldContent.classList.add('hidden');
    if (fieldAttachments) fieldAttachments.classList.add('hidden');
    if (fieldMenuOptions) fieldMenuOptions.classList.add('hidden');

    if (type === 'ticket') {
        fieldGLPI.classList.remove('hidden');
        fieldContent.classList.remove('hidden');
        if (fieldAttachments) fieldAttachments.classList.remove('hidden');
        labelContent.innerText = "Prompt de Descrição (Opcional)";
        document.getElementById('node-content').placeholder = "Ex: por favor, descreva o problema detalhadamente:";
    } else if (type === 'text') {
        fieldContent.classList.remove('hidden');
        labelContent.innerText = "Mensagem de Resposta (FAQ)";
        document.getElementById('node-content').placeholder = "Escreva a resposta automática que o usuário receberá...";
    } else if (type === 'menu') {
        const isRoot = activeEditMode === "edit" && activeEditPath.length === 0;
        if (!isRoot && fieldMenuOptions) {
            fieldMenuOptions.classList.remove('hidden');
        }
    }
}

// Confirma as alterações no modal
function confirmModal() {
    const title = document.getElementById('node-title').value.trim();
    const type = document.getElementById('node-type').value;
    const glpi_id = parseInt(document.getElementById('node-glpi').value) || 0;
    const content = document.getElementById('node-content').value.trim();

    if (!title) {
        alert('O título da opção é obrigatório.');
        return;
    }

    if (activeEditMode === "edit") {
        const node = funcNodeByPath(activeEditPath);
        node.title = title;
        node.type = type;
        node.glpi_id = glpi_id;
        node.content = content;
        
        if (type === 'ticket') {
            node.ask_images = document.getElementById('node-ask-images').checked;
            node.ask_docs = document.getElementById('node-ask-docs').checked;
        } else {
            delete node.ask_images;
            delete node.ask_docs;
        }

        const isRoot = activeEditPath.length === 0;
        if (type === 'menu' && !isRoot) {
            node.show_back_button = document.getElementById('node-show-back').checked;
        } else {
            delete node.show_back_button;
        }

        // Limpar filhos se mudou de menu para outro tipo
        if (type !== 'menu') delete node.children;
    } else if (activeEditMode === "add") {
        const parent = funcNodeByPath(activeEditPath);
        if (!parent.children) parent.children = [];
        
        // Gerar um ID único simples
        const newID = "node_" + Math.random().toString(36).substr(2, 9);
        
        const newNode = {
            id: newID,
            title,
            type,
            glpi_id,
            content
        };
        
        if (type === 'ticket') {
            newNode.ask_images = document.getElementById('node-ask-images').checked;
            newNode.ask_docs = document.getElementById('node-ask-docs').checked;
        }
        
        if (type === 'menu') {
            newNode.show_back_button = document.getElementById('node-show-back').checked;
            newNode.children = [];
        }
        
        parent.children.push(newNode);
    }

    closeModal();
    renderTree();
}

// Exclui um nó
function deleteNode(parentPath, idx) {
    if (!confirm('Deseja realmente remover esta opção e todos os seus submenus?')) return;
    const parent = funcNodeByPath(parentPath);
    parent.children.splice(idx, 1);
    renderTree();
}

// Move a posição do nó para cima ou para baixo
function moveNode(parentPath, idx, direction) {
    const parent = funcNodeByPath(parentPath);
    const targetIdx = idx + direction;

    if (targetIdx < 0 || targetIdx >= parent.children.length) return;

    const temp = parent.children[idx];
    parent.children[idx] = parent.children[targetIdx];
    parent.children[targetIdx] = temp;

    renderTree();
}

// Salva a árvore na API
async function saveFlow() {
    try {
        const response = await fetch('/api/flow', {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json'
            },
            body: JSON.stringify(flowRoot)
        });

        if (!response.ok) throw new Error('Erro ao salvar fluxo');
        showToast('Árvore de fluxo salva e aplicada com sucesso!', '✅');
    } catch (err) {
        console.error(err);
        showToast('Erro ao salvar o fluxo.', '❌');
    }
}

// Toast visual
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

window.addEventListener('DOMContentLoaded', fetchFlow);
