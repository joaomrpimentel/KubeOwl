document.addEventListener('DOMContentLoaded', () => {
    const app = new KubeOwlApp();
    app.init();
});

class KubeOwlApp {
    constructor() {
        this.currentSort = { key: 'name', order: 'asc' };
        this.dataCache = {
            pods: [],
            nodes: [],
            services: [],
            ingresses: [],
            pvcs: [],
            events: [],
            overview: {}
        };
        this.ws = null;
        // Mapeia UID do recurso para o elemento do DOM para atualizações rápidas
        this.domElementMap = new Map(); 
    }

    init() {
        this.setupTheme();
        this.setupNavigation();
        this.fetchInitialData();
        this.setupWebSocket();
        // Atualiza as métricas (CPU/Mem) periodicamente, já que não vêm pelo watch
        setInterval(() => this.fetchMetrics(), 30000);
    }
    
    setupTheme() {
        const themeToggle = document.getElementById('theme-toggle');
        const sunIcon = '<i class="fas fa-sun"></i>';
        const moonIcon = '<i class="fas fa-moon"></i>';

        const applyTheme = (theme) => {
            if (theme === 'dark') {
                document.documentElement.classList.add('dark');
                themeToggle.innerHTML = sunIcon;
            } else {
                document.documentElement.classList.remove('dark');
                themeToggle.innerHTML = moonIcon;
            }
        };

        const storedTheme = localStorage.getItem('theme');
        const preferredTheme = window.matchMedia('(prefers-color-scheme: dark)').matches ? 'dark' : 'light';
        const currentTheme = storedTheme || preferredTheme;
        applyTheme(currentTheme);

        themeToggle.addEventListener('click', () => {
            const isDark = document.documentElement.classList.toggle('dark');
            const newTheme = isDark ? 'dark' : 'light';
            localStorage.setItem('theme', newTheme);
            applyTheme(newTheme);
        });
    }
    
    setupNavigation() {
        const navLinks = document.querySelectorAll('.nav-link');
        const sections = document.querySelectorAll('.main-section');
        navLinks.forEach(link => {
            link.addEventListener('click', (e) => {
                e.preventDefault();
                const targetId = link.getAttribute('href').substring(1);

                navLinks.forEach(l => l.classList.remove('active'));
                link.classList.add('active');

                sections.forEach(s => {
                    s.classList.toggle('hidden', s.id !== `${targetId}-section`);
                });
                // Re-renderiza a partir do cache ao mudar de aba para garantir consistência
                this.renderAllSections();
            });
        });
    }

    // Busca todos os dados uma vez para popular a UI rapidamente
    async fetchInitialData() {
        const endpoints = ['overview', 'nodes', 'pods', 'services', 'ingresses', 'pvcs', 'events'];
        try {
            const promises = endpoints.map(e => fetch(`/api/${e}`).then(res => res.json()));
            const [overview, nodes, pods, services, ingresses, pvcs, events] = await Promise.all(promises);
            
            this.dataCache = { overview, nodes, pods, services, ingresses, pvcs, events };
            
            document.getElementById('last-updated').innerText = `Carregado: ${new Date().toLocaleTimeString()}`;
            this.renderAllSections();
        } catch (error) {
            console.error("Erro ao buscar dados iniciais:", error);
            document.getElementById('last-updated').innerText = "Erro ao carregar dados.";
        }
    }

    // Busca apenas os dados de métricas periodicamente
    async fetchMetrics() {
        try {
            const [nodes, pods] = await Promise.all([
                fetch('/api/nodes').then(res => res.json()),
                fetch('/api/pods').then(res => res.json())
            ]);

            // Atualiza o cache de nós e pods com as novas métricas
            this.dataCache.nodes = nodes;
            this.dataCache.pods = pods;
            
            // Re-renderiza as seções afetadas
            this.renderNodeList(this.dataCache.nodes);
            this.renderPodTable(this.dataCache.pods);
        } catch (error) {
            console.error("Erro ao buscar métricas:", error);
        }
    }

    setupWebSocket() {
        const wsProtocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
        this.ws = new WebSocket(`${wsProtocol}//${window.location.host}/ws`);

        this.ws.onopen = () => {
            console.log('Conectado ao servidor WebSocket.');
            document.getElementById('update-indicator').style.backgroundColor = 'var(--green-500)';
        };

        this.ws.onmessage = (event) => {
            const message = JSON.parse(event.data);
            this.handleWebSocketMessage(message);
        };

        this.ws.onclose = () => {
            console.log('Desconectado. Tentando reconectar em 5 segundos...');
            document.getElementById('update-indicator').style.backgroundColor = 'var(--red-500)';
            setTimeout(() => this.setupWebSocket(), 5000);
        };

        this.ws.onerror = (error) => {
            console.error('Erro no WebSocket:', error);
            this.ws.close();
        };
    }

    handleWebSocketMessage(message) {
        const { type, payload } = message;
        const resource = payload.object;
        const eventType = payload.type; // ADDED, MODIFIED, DELETED

        document.getElementById('last-updated').innerText = `Atualizado: ${new Date().toLocaleTimeString()}`;
        const indicator = document.getElementById('update-indicator');
        indicator.style.backgroundColor = 'var(--blue-500)';
        setTimeout(() => { indicator.style.backgroundColor = 'var(--green-500)'; }, 500);

        switch (type) {
            case 'pods':
                this.updateAndRenderResource({
                    resource,
                    eventType,
                    cacheKey: 'pods',
                    idKey: 'uid', // UID é o identificador único
                    renderFunction: this.renderPodRow,
                    containerId: 'pods-table-body',
                });
                break;
            case 'events':
                 // Eventos são sempre adicionados
                const eventInfo = this.processEventInfo(resource);
                this.dataCache.events.unshift(eventInfo);
                if (this.dataCache.events.length > 50) this.dataCache.events.pop();
                
                // Renderiza apenas o novo evento no topo
                const eventsList = document.getElementById('events-list');
                const newEventElement = this.createEventCard(eventInfo);
                eventsList.prepend(newEventElement);
                if (eventsList.children.length > 50) eventsList.lastChild.remove();
                break;
            // Lógica para 'nodes' pode ser adicionada aqui
        }
    }
    
    // Função genérica para atualizar cache e DOM
    updateAndRenderResource({ resource, eventType, cacheKey, idKey, renderFunction, containerId }) {
        const id = resource.metadata[idKey];
        const cache = this.dataCache[cacheKey];
        const existingIndex = cache.findIndex(item => item[idKey] === id);
        const container = document.getElementById(containerId);

        // Lógica de remoção
        if (eventType === 'DELETED') {
            if (existingIndex > -1) cache.splice(existingIndex, 1);
            const domElement = this.domElementMap.get(id);
            if (domElement) {
                domElement.remove();
                this.domElementMap.delete(id);
            }
            return;
        }

        // Processa o item para o formato do frontend (simplificado)
        const processedItem = this.processRawResource(resource, cacheKey);

        // Lógica de Adição/Modificação
        if (existingIndex > -1) {
            // Modifica: Mantém os dados antigos (como métricas) e atualiza com os novos do WebSocket
            cache[existingIndex] = { ...cache[existingIndex], ...processedItem };
            const domElement = this.domElementMap.get(id);
            if (domElement) {
                // Atualiza o conteúdo do elemento existente
                const newElement = renderFunction.call(this, cache[existingIndex]);
                domElement.innerHTML = newElement.innerHTML;
            }
        } else {
            // Adiciona
            cache.unshift(processedItem); // Adiciona no início
            const newElement = renderFunction.call(this, processedItem);
            container.prepend(newElement);
            this.domElementMap.set(id, newElement);
        }
    }
    
    // Converte recurso bruto do watch para o formato esperado pelo cache
    processRawResource(resource, type) {
        if (type === 'pods') {
            const status = resource.status.containerStatuses?.find(cs => cs.state.waiting)?.state.waiting.reason || resource.status.phase;
            const restarts = resource.status.containerStatuses?.reduce((sum, cs) => sum + cs.restartCount, 0) || 0;
            return {
                uid: resource.metadata.uid,
                name: resource.metadata.name,
                namespace: resource.metadata.namespace,
                nodeName: resource.spec.nodeName,
                status: status,
                restarts: restarts,
            };
        }
        return resource;
    }
    
    processEventInfo(event) {
        return {
            timestamp: new Date(event.lastTimestamp || event.eventTime).toLocaleString(),
            type: event.type,
            reason: event.reason,
            object: `${event.involvedObject.kind}/${event.involvedObject.name}`,
            message: event.message,
        };
    }
    
    // Renderiza todas as seções com os dados do cache
    renderAllSections() {
        this.renderOverview(this.dataCache.overview);
        this.renderNodeList(this.dataCache.nodes);
        this.renderPodTable(this.dataCache.pods);
        this.renderServicesView(this.dataCache.services);
        this.renderIngressesView(this.dataCache.ingresses);
        this.renderEventFeed(this.dataCache.events);
        this.renderStorageView(this.dataCache.pvcs);
    }

    renderOverview(data) {
        if (!data || !Object.keys(data).length) return;
        document.getElementById('running-status').innerHTML = data.isRunningInCluster 
            ? `<span style="color: var(--green-500);">In-Cluster</span>` 
            : `<span style="color: var(--yellow-500);">Local</span>`;
        document.getElementById('nodes-count').innerText = data.nodeCount || 0;
        document.getElementById('deployments-count').innerText = data.deploymentCount || 0;
        document.getElementById('namespaces-count').innerText = data.namespaceCount || 0;
        this.renderCapacityView(data.capacity);
    }

    renderCapacityView(capacity) {
        if (!capacity) return;
        const toGiB = (bytes) => (bytes / (1024 * 1024 * 1024)).toFixed(2);
        const toCores = (milli) => (milli / 1000).toFixed(2);
        
        document.getElementById('cpu-progress-bar').style.width = `${capacity.cpuUsagePercentage.toFixed(2)}%`;
        document.getElementById('cpu-usage-text').innerText = `${toCores(capacity.usedCpu)} / ${toCores(capacity.totalCpu)} Cores`;
        document.getElementById('cpu-usage-percentage').innerText = `${capacity.cpuUsagePercentage.toFixed(2)}%`;
        
        document.getElementById('memory-progress-bar').style.width = `${capacity.memoryUsagePercentage.toFixed(2)}%`;
        document.getElementById('memory-usage-text').innerText = `${toGiB(capacity.usedMemory)} / ${toGiB(capacity.totalMemory)} GiB`;
        document.getElementById('memory-usage-percentage').innerText = `${capacity.memoryUsagePercentage.toFixed(2)}%`;
    }

    renderNodeList(nodes) {
        const container = document.getElementById('nodes-list');
        container.innerHTML = nodes.map(node => `
            <div class="card node-card" id="node-${node.name}">
                <div class="node-header">
                    <h4>${node.name}</h4>
                    ${node.role === 'Control-Plane' ? '<span class="node-role">MASTER</span>' : ''}
                </div>
                <div>
                    <div class="node-metric-label">
                        <span>CPU</span>
                        <span style="font-family: monospace;">${node.usedCpu} / ${node.totalCpu} Cores</span>
                    </div>
                    <div class="progress-bar-bg"><div class="progress-bar bg-blue" style="width: ${node.cpuUsagePercentage.toFixed(2)}%"></div></div>
                </div>
                <div>
                    <div class="node-metric-label">
                        <span>Memória</span>
                        <span style="font-family: monospace;">${node.usedMemory} / ${node.totalMemory}</span>
                    </div>
                    <div class="progress-bar-bg"><div class="progress-bar bg-green" style="width: ${node.memoryUsagePercentage.toFixed(2)}%"></div></div>
                </div>
                <div class="node-pods-count">
                     <span>Pods em Execução</span>
                     <span class="count">${node.podCount}</span>
                </div>
            </div>
        `).join('');
    }

    getPodStatusClass(status) {
        if (!status) return 'status-unknown';
        const s = status.toLowerCase();
        if (s.includes('running') || s.includes('succeeded')) return 'status-running';
        if (s.includes('pending') || s.includes('creating')) return 'status-pending';
        if (s.includes('failed') || s.includes('error') || s.includes('crash')) return 'status-failed';
        return 'status-unknown';
    }

    // Cria um elemento <tr> para um pod
    renderPodRow(pod) {
        const tr = document.createElement('tr');
        tr.id = `pod-${pod.uid}`;
        tr.innerHTML = `
            <td><div><b>${pod.name}</b></div><div style="font-size: 0.8rem; color: var(--gray-500);">${pod.namespace}</div></td>
            <td style="font-family: monospace;">${pod.nodeName || 'N/A'}</td>
            <td><span class="status-badge ${this.getPodStatusClass(pod.status)}">${pod.status || 'Unknown'}</span></td>
            <td style="font-family: monospace; text-align: center;">${pod.restarts}</td>
            <td style="font-family: monospace;">${pod.usedCpu || '-'}</td>
            <td style="font-family: monospace;">${pod.usedMemory || '-'}</td>
        `;
        return tr;
    }
    
    renderPodTable(pods) {
        const tableHeader = document.getElementById('pods-table-header');
        const tableBody = document.getElementById('pods-table-body');

        // Adiciona a lógica para renderizar o cabeçalho e os listeners de ordenação
        const headers = [
            { name: 'Pod / Namespace', key: 'name' },
            { name: 'Nó', key: 'nodeName' },
            { name: 'Status', key: 'status' },
            { name: 'Restarts', key: 'restarts' },
            { name: 'CPU', key: 'usedCpuMilli' },
            { name: 'Memória', key: 'usedMemoryBytes' },
        ];
        
        tableHeader.innerHTML = headers.map(h => 
            `<th data-key="${h.key}" style="cursor: pointer;">${h.name} ${this.currentSort.key === h.key ? (this.currentSort.order === 'asc' ? '▲' : '▼') : ''}</th>`
        ).join('');

        document.querySelectorAll('#pods-table-header th').forEach(th => {
            th.addEventListener('click', () => {
                const key = th.dataset.key;
                if (this.currentSort.key === key) {
                    this.currentSort.order = this.currentSort.order === 'asc' ? 'desc' : 'asc';
                } else {
                    this.currentSort.key = key;
                    this.currentSort.order = 'asc';
                }
                this.renderPodTable(this.dataCache.pods);
            });
        });

        tableBody.innerHTML = ''; // Limpa a tabela
        this.domElementMap.clear(); // Limpa o mapa de elementos

        // Ordena os pods
        pods.sort((a, b) => {
            let aVal = a[this.currentSort.key]; 
            let bVal = b[this.currentSort.key];
            
            if (aVal === undefined || aVal === null) aVal = typeof bVal === 'string' ? '' : 0;
            if (bVal === undefined || bVal === null) bVal = typeof aVal === 'string' ? '' : 0;

            if (typeof aVal === 'string') {
                return this.currentSort.order === 'asc' ? aVal.localeCompare(bVal) : bVal.localeCompare(aVal);
            }
            return this.currentSort.order === 'asc' ? aVal - bVal : bVal - aVal;
        });

        if (pods.length === 0) {
            tableBody.innerHTML = '<tr><td colspan="6" style="text-align: center; padding: 2rem;">Nenhum pod encontrado.</td></tr>';
            return;
        }
        
        // Renderiza as linhas da tabela
        pods.forEach(pod => {
            const row = this.renderPodRow(pod);
            tableBody.appendChild(row);
            this.domElementMap.set(pod.uid, row);
        });
    }

    renderServicesView(services) {
        const servicesTableBody = document.getElementById('services-table-body');
        servicesTableBody.innerHTML = services.length ? services.map(service => `
             <tr>
                <td>${service.namespace}</td>
                <td><b>${service.name}</b></td>
                <td style="font-family: monospace;">${service.type}</td>
                <td style="font-family: monospace;">${service.clusterIp || 'N/A'}</td>
                <td style="font-family: monospace;">${service.externalIp || 'N/A'}</td>
                <td style="font-family: monospace;">${service.ports}</td>
            </tr>`
        ).join('') : '<tr><td colspan="6" style="text-align: center; padding: 2rem;">Nenhum Serviço encontrado.</td></tr>';
    }

    renderIngressesView(ingresses) {
        const ingressesTableBody = document.getElementById('ingresses-table-body');
        ingressesTableBody.innerHTML = ingresses.length ? ingresses.map(ingress => `
             <tr>
                <td>${ingress.namespace}</td>
                <td><b>${ingress.name}</b></td>
                <td style="font-family: monospace;"><a href="http://${ingress.hosts.split(',')[0]}" target="_blank" style="color: var(--blue-500); text-decoration: none;">${ingress.hosts}</a></td>
                <td style="font-family: monospace;">${ingress.service}</td>
            </tr>`
        ).join('') : '<tr><td colspan="4" style="text-align: center; padding: 2rem;">Nenhum Ingress encontrado.</td></tr>';
    }
    
    createEventCard(event) {
        const div = document.createElement('div');
        div.className = 'card event-card';
        const eventTypeBorders = { 'Normal': 'var(--blue-500)', 'Warning': 'var(--yellow-500)' };
        div.style.borderLeftColor = eventTypeBorders[event.type] || 'var(--gray-500)';
        div.innerHTML = `
            <div class="event-header">
                <b>${event.reason}</b>
                <span>${event.timestamp}</span>
            </div>
            <p class="event-message"><b>${event.object}:</b> ${event.message}</p>`;
        return div;
    }

    renderEventFeed(events) {
        const eventsList = document.getElementById('events-list');
        eventsList.innerHTML = '';
        if (events.length) {
            events.forEach(event => {
                eventsList.appendChild(this.createEventCard(event));
            });
        } else {
            eventsList.innerHTML = '<p>Nenhum evento recente.</p>';
        }
    }

    renderStorageView(pvcs) {
        const pvcsTableBody = document.getElementById('pvcs-table-body');
        pvcsTableBody.innerHTML = pvcs.length ? pvcs.map(pvc => `
             <tr>
                <td>${pvc.namespace}</td>
                <td><b>${pvc.name}</b></td>
                <td><span class="status-badge ${pvc.status === 'Bound' ? 'status-bound' : 'status-pending'}">${pvc.status}</span></td>
                <td style="font-family: monospace;">${pvc.capacity}</td>
            </tr>`
        ).join('') : '<tr><td colspan="4" style="text-align: center; padding: 2rem;">Nenhum PVC encontrado.</td></tr>';
    }
}
