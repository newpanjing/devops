let token = localStorage.getItem('token');
let currentPage = 'dashboard';
let dbConfigs = [];
let sourceConfig = null;
let targetConfigs = [];
let schemaDiff = null;
let dataSyncResults = null;

async function init() {
    if (token) {
        const valid = await validateToken();
        if (valid) {
            renderApp();
        } else {
            renderLogin();
        }
    } else {
        renderLogin();
    }
}

async function validateToken() {
    try {
        const response = await fetch('/api/auth/validate', {
            headers: { 'Authorization': `Bearer ${token}` }
        });
        const result = await response.json();
        return result.valid;
    } catch {
        return false;
    }
}

function renderLogin() {
    document.getElementById('app').innerHTML = `
        <div class="min-h-screen flex items-center justify-center bg-gradient-to-br from-blue-600 to-indigo-700">
            <div class="bg-white rounded-xl shadow-2xl p-8 w-full max-w-md">
                <div class="text-center mb-6">
                    <div class="flex justify-center mb-4">
                        <div class="w-16 h-16 bg-blue-100 rounded-full flex items-center justify-center">
                            <i class="fa fa-database text-blue-600 text-3xl"></i>
                        </div>
                    </div>
                    <h1 class="text-2xl font-bold text-gray-800">数据库同步工具</h1>
                    <p class="text-gray-500 mt-2">请登录以继续</p>
                </div>
                <form id="loginForm" class="space-y-4">
                    <div>
                        <label class="block text-sm font-medium text-gray-700 mb-1">用户名</label>
                        <input type="text" name="username" class="w-full px-4 py-2 border border-gray-300 rounded-lg focus:ring-2 focus:ring-blue-500 focus:border-transparent outline-none transition" placeholder="请输入用户名">
                    </div>
                    <div>
                        <label class="block text-sm font-medium text-gray-700 mb-1">密码</label>
                        <input type="password" name="password" class="w-full px-4 py-2 border border-gray-300 rounded-lg focus:ring-2 focus:ring-blue-500 focus:border-transparent outline-none transition" placeholder="请输入密码">
                    </div>
                    <button type="submit" class="w-full bg-blue-600 text-white py-2 rounded-lg font-medium hover:bg-blue-700 transition flex items-center justify-center">
                        <i class="fa fa-sign-in mr-2"></i>登录
                    </button>
                </form>
                <div id="loginError" class="mt-4 text-red-500 text-sm text-center hidden"></div>
                <p class="mt-6 text-center text-gray-500 text-sm">
                    默认账号: admin / admin123
                </p>
            </div>
        </div>
    `;
    document.getElementById('loginForm').addEventListener('submit', handleLogin);
}

async function handleLogin(e) {
    e.preventDefault();
    const form = e.target;
    const data = {
        username: form.username.value,
        password: form.password.value
    };
    try {
        const response = await fetch('/api/auth/login', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify(data)
        });
        const result = await response.json();
        if (result.success) {
            token = result.token;
            localStorage.setItem('token', token);
            renderApp();
        } else {
            document.getElementById('loginError').textContent = result.message;
            document.getElementById('loginError').classList.remove('hidden');
        }
    } catch (err) {
        document.getElementById('loginError').textContent = '登录失败，请重试';
        document.getElementById('loginError').classList.remove('hidden');
    }
}

function renderApp() {
    document.getElementById('app').innerHTML = `
        <div class="flex">
            <aside class="sidebar bg-gray-800 text-white w-64 fixed h-full">
                <div class="p-4 border-b border-gray-700">
                    <div class="flex items-center">
                        <i class="fa fa-database text-blue-400 text-xl mr-2"></i>
                        <span class="font-bold text-lg">数据库同步</span>
                    </div>
                </div>
                <nav class="mt-4">
                    <a href="#" onclick="navigate('dashboard')" class="nav-link ${currentPage === 'dashboard' ? 'bg-gray-700' : ''} flex items-center px-4 py-3 hover:bg-gray-700 transition">
                        <i class="fa fa-home mr-3"></i>仪表盘
                    </a>
                    <a href="#" onclick="navigate('dbconfig')" class="nav-link ${currentPage === 'dbconfig' ? 'bg-gray-700' : ''} flex items-center px-4 py-3 hover:bg-gray-700 transition">
                        <i class="fa fa-cog mr-3"></i>数据库配置
                    </a>
                    <a href="#" onclick="navigate('schemacompare')" class="nav-link ${currentPage === 'schemacompare' ? 'bg-gray-700' : ''} flex items-center px-4 py-3 hover:bg-gray-700 transition">
                        <i class="fa fa-table mr-3"></i>表结构比对
                    </a>
                    <a href="#" onclick="navigate('datasync')" class="nav-link ${currentPage === 'datasync' ? 'bg-gray-700' : ''} flex items-center px-4 py-3 hover:bg-gray-700 transition">
                        <i class="fa fa-refresh mr-3"></i>数据同步
                    </a>
                    <a href="#" onclick="navigate('audit')" class="nav-link ${currentPage === 'audit' ? 'bg-gray-700' : ''} flex items-center px-4 py-3 hover:bg-gray-700 transition">
                        <i class="fa fa-history mr-3"></i>审计日志
                    </a>
                </nav>
                <div class="absolute bottom-0 w-full p-4 border-t border-gray-700">
                    <button onclick="logout()" class="w-full text-left flex items-center px-4 py-2 hover:bg-gray-700 transition text-gray-400">
                        <i class="fa fa-sign-out mr-3"></i>退出登录
                    </button>
                </div>
            </aside>
            <main class="ml-64 p-6 w-full">
                <div id="mainContent"></div>
            </main>
        </div>
    `;
    renderMainContent();
}

function logout() {
    token = null;
    localStorage.removeItem('token');
    renderLogin();
}

function navigate(page) {
    currentPage = page;
    renderApp();
}

async function renderMainContent() {
    const content = document.getElementById('mainContent');
    switch (currentPage) {
        case 'dashboard':
            renderDashboard(content);
            break;
        case 'dbconfig':
            await renderDBConfig(content);
            break;
        case 'schemacompare':
            await renderSchemaCompare(content);
            break;
        case 'datasync':
            await renderDataSync(content);
            break;
        case 'audit':
            await renderAudit(content);
            break;
    }
}

function renderDashboard(content) {
    content.innerHTML = `
        <div class="mb-6">
            <h1 class="text-2xl font-bold text-gray-800">仪表盘</h1>
            <p class="text-gray-500 mt-1">欢迎使用数据库同步工具</p>
        </div>
        <div class="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-6">
            <div class="bg-white rounded-xl shadow-sm p-6">
                <div class="flex items-center">
                    <div class="w-12 h-12 bg-blue-100 rounded-lg flex items-center justify-center">
                        <i class="fa fa-database text-blue-600 text-xl"></i>
                    </div>
                    <div class="ml-4">
                        <p class="text-gray-500 text-sm">数据库配置</p>
                        <p class="text-2xl font-bold text-gray-800">${dbConfigs.length}</p>
                    </div>
                </div>
            </div>
            <div class="bg-white rounded-xl shadow-sm p-6">
                <div class="flex items-center">
                    <div class="w-12 h-12 bg-green-100 rounded-lg flex items-center justify-center">
                        <i class="fa fa-table text-green-600 text-xl"></i>
                    </div>
                    <div class="ml-4">
                        <p class="text-gray-500 text-sm">表结构比对</p>
                        <p class="text-2xl font-bold text-gray-800">0</p>
                    </div>
                </div>
            </div>
            <div class="bg-white rounded-xl shadow-sm p-6">
                <div class="flex items-center">
                    <div class="w-12 h-12 bg-yellow-100 rounded-lg flex items-center justify-center">
                        <i class="fa fa-refresh text-yellow-600 text-xl"></i>
                    </div>
                    <div class="ml-4">
                        <p class="text-gray-500 text-sm">数据同步</p>
                        <p class="text-2xl font-bold text-gray-800">0</p>
                    </div>
                </div>
            </div>
            <div class="bg-white rounded-xl shadow-sm p-6">
                <div class="flex items-center">
                    <div class="w-12 h-12 bg-purple-100 rounded-lg flex items-center justify-center">
                        <i class="fa fa-history text-purple-600 text-xl"></i>
                    </div>
                    <div class="ml-4">
                        <p class="text-gray-500 text-sm">审计日志</p>
                        <p class="text-2xl font-bold text-gray-800">0</p>
                    </div>
                </div>
            </div>
        </div>
        <div class="mt-6 bg-white rounded-xl shadow-sm p-6">
            <h2 class="text-lg font-semibold text-gray-800 mb-4">快速开始</h2>
            <div class="grid grid-cols-1 md:grid-cols-3 gap-4">
                <button onclick="navigate('dbconfig')" class="p-4 border border-gray-200 rounded-lg hover:border-blue-500 hover:bg-blue-50 transition flex flex-col items-center">
                    <i class="fa fa-cog text-blue-500 text-2xl mb-2"></i>
                    <span class="font-medium text-gray-700">添加数据库配置</span>
                    <span class="text-sm text-gray-500">配置源和目标数据库</span>
                </button>
                <button onclick="navigate('schemacompare')" class="p-4 border border-gray-200 rounded-lg hover:border-green-500 hover:bg-green-50 transition flex flex-col items-center">
                    <i class="fa fa-table text-green-500 text-2xl mb-2"></i>
                    <span class="font-medium text-gray-700">比对表结构</span>
                    <span class="text-sm text-gray-500">查看源和目标表差异</span>
                </button>
                <button onclick="navigate('datasync')" class="p-4 border border-gray-200 rounded-lg hover:border-yellow-500 hover:bg-yellow-50 transition flex flex-col items-center">
                    <i class="fa fa-refresh text-yellow-500 text-2xl mb-2"></i>
                    <span class="font-medium text-gray-700">同步数据</span>
                    <span class="text-sm text-gray-500">将数据同步到目标数据库</span>
                </button>
            </div>
        </div>
    `;
}

async function renderDBConfig(content) {
    await loadDBConfigs();
    content.innerHTML = `
        <div class="mb-6 flex justify-between items-center">
            <div>
                <h1 class="text-2xl font-bold text-gray-800">数据库配置</h1>
                <p class="text-gray-500 mt-1">管理数据库连接配置</p>
            </div>
            <button onclick="showAddConfigModal()" class="bg-blue-600 text-white px-4 py-2 rounded-lg font-medium hover:bg-blue-700 transition flex items-center">
                <i class="fa fa-plus mr-2"></i>添加配置
            </button>
        </div>
        <div class="bg-white rounded-xl shadow-sm p-6">
            <div class="overflow-x-auto">
                <table class="w-full">
                    <thead>
                        <tr class="border-b border-gray-200">
                            <th class="text-left py-3 px-4 font-semibold text-gray-600">名称</th>
                            <th class="text-left py-3 px-4 font-semibold text-gray-600">类型</th>
                            <th class="text-left py-3 px-4 font-semibold text-gray-600">主机</th>
                            <th class="text-left py-3 px-4 font-semibold text-gray-600">数据库</th>
                            <th class="text-left py-3 px-4 font-semibold text-gray-600">操作</th>
                        </tr>
                    </thead>
                    <tbody>
                        ${dbConfigs.map(config => `
                            <tr class="border-b border-gray-100 hover:bg-gray-50">
                                <td class="py-3 px-4 text-gray-800">${config.name}</td>
                                <td class="py-3 px-4">
                                    <span class="px-2 py-1 text-xs rounded-full ${getDBTypeClass(config.db_type)}">${config.db_type}</span>
                                </td>
                                <td class="py-3 px-4 text-gray-600">${config.host}:${config.port}</td>
                                <td class="py-3 px-4 text-gray-600">${config.database}</td>
                                <td class="py-3 px-4">
                                    <button onclick="testConnection(${config.id})" class="text-blue-600 hover:text-blue-800 mr-3">
                                        <i class="fa fa-check-circle"></i>
                                    </button>
                                    <button onclick="deleteConfig(${config.id})" class="text-red-600 hover:text-red-800">
                                        <i class="fa fa-trash"></i>
                                    </button>
                                </td>
                            </tr>
                        `).join('')}
                        ${dbConfigs.length === 0 ? `
                            <tr>
                                <td colspan="5" class="py-8 text-center text-gray-500">暂无数据库配置</td>
                            </tr>
                        ` : ''}
                    </tbody>
                </table>
            </div>
        </div>
        <div id="configModal" class="fixed inset-0 bg-black bg-opacity-50 flex items-center justify-center hidden z-50">
            <div class="bg-white rounded-xl p-6 w-full max-w-lg">
                <div class="flex justify-between items-center mb-4">
                    <h2 class="text-xl font-bold text-gray-800">添加数据库配置</h2>
                    <button onclick="hideAddConfigModal()" class="text-gray-400 hover:text-gray-600">
                        <i class="fa fa-times text-xl"></i>
                    </button>
                </div>
                <form id="configForm" class="space-y-4">
                    <div>
                        <label class="block text-sm font-medium text-gray-700 mb-1">配置名称</label>
                        <input type="text" name="name" class="w-full px-4 py-2 border border-gray-300 rounded-lg focus:ring-2 focus:ring-blue-500 outline-none" required placeholder="例如: 生产数据库">
                    </div>
                    <div>
                        <label class="block text-sm font-medium text-gray-700 mb-1">数据库类型</label>
                        <select name="db_type" class="w-full px-4 py-2 border border-gray-300 rounded-lg focus:ring-2 focus:ring-blue-500 outline-none" required>
                            <option value="mysql">MySQL</option>
                            <option value="postgres">PostgreSQL</option>
                            <option value="sqlite">SQLite</option>
                        </select>
                    </div>
                    <div>
                        <label class="block text-sm font-medium text-gray-700 mb-1">主机</label>
                        <input type="text" name="host" class="w-full px-4 py-2 border border-gray-300 rounded-lg focus:ring-2 focus:ring-blue-500 outline-none" placeholder="localhost">
                    </div>
                    <div>
                        <label class="block text-sm font-medium text-gray-700 mb-1">端口</label>
                        <input type="number" name="port" class="w-full px-4 py-2 border border-gray-300 rounded-lg focus:ring-2 focus:ring-blue-500 outline-none" placeholder="3306">
                    </div>
                    <div>
                        <label class="block text-sm font-medium text-gray-700 mb-1">数据库名/路径</label>
                        <input type="text" name="database" class="w-full px-4 py-2 border border-gray-300 rounded-lg focus:ring-2 focus:ring-blue-500 outline-none" required placeholder="MySQL/PostgreSQL填数据库名，SQLite填文件路径">
                    </div>
                    <div>
                        <label class="block text-sm font-medium text-gray-700 mb-1">用户名</label>
                        <input type="text" name="username" class="w-full px-4 py-2 border border-gray-300 rounded-lg focus:ring-2 focus:ring-blue-500 outline-none" placeholder="root">
                    </div>
                    <div>
                        <label class="block text-sm font-medium text-gray-700 mb-1">密码</label>
                        <input type="password" name="password" class="w-full px-4 py-2 border border-gray-300 rounded-lg focus:ring-2 focus:ring-blue-500 outline-none" placeholder="密码">
                    </div>
                    <div class="flex space-x-4">
                        <button type="button" onclick="testNewConnection()" class="flex-1 bg-gray-100 text-gray-700 py-2 rounded-lg font-medium hover:bg-gray-200 transition">
                            测试连接
                        </button>
                        <button type="submit" class="flex-1 bg-blue-600 text-white py-2 rounded-lg font-medium hover:bg-blue-700 transition">
                            保存配置
                        </button>
                    </div>
                </form>
            </div>
        </div>
    `;
    document.getElementById('configForm').addEventListener('submit', handleSaveConfig);
}

function getDBTypeClass(dbType) {
    switch(dbType) {
        case 'mysql': return 'bg-orange-100 text-orange-700';
        case 'postgres': return 'bg-blue-100 text-blue-700';
        case 'sqlite': return 'bg-gray-100 text-gray-700';
        default: return 'bg-gray-100 text-gray-700';
    }
}

function showAddConfigModal() {
    document.getElementById('configModal').classList.remove('hidden');
}

function hideAddConfigModal() {
    document.getElementById('configModal').classList.add('hidden');
    document.getElementById('configForm').reset();
}

async function handleSaveConfig(e) {
    e.preventDefault();
    const form = e.target;
    const formData = new FormData(form);
    const data = {
        name: formData.get('name'),
        db_type: formData.get('db_type'),
        host: formData.get('host'),
        port: parseInt(formData.get('port') || '0'),
        database: formData.get('database'),
        username: formData.get('username'),
        password: formData.get('password')
    };
    try {
        const response = await fetch('/api/dbconfig', {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json',
                'Authorization': `Bearer ${token}`
            },
            body: JSON.stringify(data)
        });
        const result = await response.json();
        if (result.message) {
            hideAddConfigModal();
            renderDBConfig(document.getElementById('mainContent'));
        }
    } catch (err) {
        alert('保存失败');
    }
}

async function testNewConnection() {
    const form = document.getElementById('configForm');
    const formData = new FormData(form);
    const data = {
        name: formData.get('name'),
        db_type: formData.get('db_type'),
        host: formData.get('host'),
        port: parseInt(formData.get('port') || '0'),
        database: formData.get('database'),
        username: formData.get('username'),
        password: formData.get('password')
    };
    try {
        const response = await fetch('/api/datasync/test', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify(data)
        });
        const result = await response.json();
        if (result.success) {
            alert('连接成功');
        } else {
            alert('连接失败: ' + result.error);
        }
    } catch (err) {
        alert('测试连接失败');
    }
}

async function testConnection(id) {
    const config = dbConfigs.find(c => c.id === id);
    if (!config) return;
    try {
        const response = await fetch('/api/datasync/test', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify(config)
        });
        const result = await response.json();
        if (result.success) {
            alert('连接成功');
        } else {
            alert('连接失败: ' + result.error);
        }
    } catch (err) {
        alert('测试连接失败');
    }
}

async function deleteConfig(id) {
    if (!confirm('确定要删除这个配置吗？')) return;
    try {
        const response = await fetch(`/api/dbconfig/${id}`, {
            method: 'DELETE',
            headers: { 'Authorization': `Bearer ${token}` }
        });
        const result = await response.json();
        if (result.message) {
            renderDBConfig(document.getElementById('mainContent'));
        }
    } catch (err) {
        alert('删除失败');
    }
}

async function loadDBConfigs() {
    try {
        const response = await fetch('/api/dbconfig', {
            headers: { 'Authorization': `Bearer ${token}` }
        });
        dbConfigs = await response.json();
    } catch {
        dbConfigs = [];
    }
}

async function renderSchemaCompare(content) {
    await loadDBConfigs();
    content.innerHTML = `
        <div class="mb-6">
            <h1 class="text-2xl font-bold text-gray-800">表结构比对</h1>
            <p class="text-gray-500 mt-1">比对源数据库和目标数据库的表结构差异</p>
        </div>
        <div class="bg-white rounded-xl shadow-sm p-6 mb-6">
            <div class="grid grid-cols-1 md:grid-cols-2 gap-6">
                <div>
                    <label class="block text-sm font-medium text-gray-700 mb-2">源数据库</label>
                    <select id="sourceConfig" class="w-full px-4 py-2 border border-gray-300 rounded-lg focus:ring-2 focus:ring-blue-500 outline-none">
                        <option value="">请选择源数据库</option>
                        ${dbConfigs.map(c => `<option value="${c.id}">${c.name}</option>`).join('')}
                    </select>
                </div>
                <div>
                    <label class="block text-sm font-medium text-gray-700 mb-2">目标数据库 (可多选)</label>
                    <select id="targetConfigs" multiple class="w-full px-4 py-2 border border-gray-300 rounded-lg focus:ring-2 focus:ring-blue-500 outline-none h-40">
                        ${dbConfigs.map(c => `<option value="${c.id}">${c.name}</option>`).join('')}
                    </select>
                </div>
            </div>
            <div class="mt-6">
                <button onclick="compareSchemas()" class="bg-blue-600 text-white px-6 py-2 rounded-lg font-medium hover:bg-blue-700 transition flex items-center">
                    <i class="fa fa-search mr-2"></i>开始比对
                </button>
            </div>
        </div>
        <div id="schemaDiffResult"></div>
    `;
}

async function compareSchemas() {
    const sourceID = parseInt(document.getElementById('sourceConfig').value);
    const targetSelect = document.getElementById('targetConfigs');
    const targetIDs = Array.from(targetSelect.selectedOptions).map(o => parseInt(o.value));

    if (!sourceID || targetIDs.length === 0) {
        alert('请选择源数据库和至少一个目标数据库');
        return;
    }

    const resultDiv = document.getElementById('schemaDiffResult');
    resultDiv.innerHTML = `<div class="text-center py-8"><i class="fa fa-spinner fa-spin text-xl text-blue-600"></i> 正在比对...</div>`;

    try {
        const response = await fetch('/api/schema/compare', {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json',
                'Authorization': `Bearer ${token}`
            },
            body: JSON.stringify({ source_id: sourceID, target_ids: targetIDs })
        });
        const results = await response.json();
        renderSchemaDiffResults(results, resultDiv);
    } catch (err) {
        resultDiv.innerHTML = `<div class="text-center py-8 text-red-500">比对失败: ${err.message}</div>`;
    }
}

function renderSchemaDiffResults(results, container) {
    if (!results || results.length === 0) {
        container.innerHTML = `<div class="text-center py-8 text-gray-500">没有比对结果</div>`;
        return;
    }

    let html = '';
    results.forEach(result => {
        html += `
            <div class="bg-white rounded-xl shadow-sm p-6 mb-6">
                <h3 class="text-lg font-semibold text-gray-800 mb-4">目标: ${result.target_name}</h3>
                ${result.error ? `<div class="text-red-500">${result.error}</div>` : renderDiff(result.diff)}
            </div>
        `;
    });
    container.innerHTML = html;
}

function renderDiff(diff) {
    if (!diff || !diff.table_diffs || diff.table_diffs.length === 0) {
        return `<div class="text-green-600 text-center py-4"><i class="fa fa-check-circle"></i> 表结构完全一致</div>`;
    }

    let html = '<div class="space-y-4">';
    diff.table_diffs.forEach(tableDiff => {
        let tableHtml = `
            <div class="diff-${tableDiff.type.toLowerCase()} p-4 rounded-lg">
                <div class="flex items-center justify-between mb-2">
                    <span class="font-semibold">${getDiffTypeLabel(tableDiff.type)} 表: ${tableDiff.table_name}</span>
                    <span class="text-xs px-2 py-1 rounded-full ${getDiffTypeBadgeClass(tableDiff.type)}">${tableDiff.type}</span>
                </div>
        `;
        if (tableDiff.column_diffs && tableDiff.column_diffs.length > 0) {
            tableHtml += '<div class="ml-4 space-y-2">';
            tableDiff.column_diffs.forEach(colDiff => {
                tableHtml += `
                    <div class="flex items-center justify-between text-sm">
                        <span>${getDiffTypeLabel(colDiff.type)} 列: ${colDiff.column_name}</span>
                        ${colDiff.source_column ? `
                            <span class="text-xs text-gray-500">
                                ${colDiff.source_column.type} ${colDiff.source_column.nullable ? 'NULL' : 'NOT NULL'}
                                ${colDiff.source_column.primary_key ? 'PRIMARY KEY' : ''}
                                ${colDiff.source_column.auto_increment ? 'AUTO_INCREMENT' : ''}
                            </span>
                        ` : ''}
                    </div>
                `;
            });
            tableHtml += '</div>';
        }
        tableHtml += '</div>';
        html += tableHtml;
    });
    html += '</div>';

    html += `
        <div class="mt-6 flex justify-end">
            <button onclick="syncSchema()" class="bg-green-600 text-white px-6 py-2 rounded-lg font-medium hover:bg-green-700 transition flex items-center">
                <i class="fa fa-refresh mr-2"></i>同步表结构
            </button>
        </div>
    `;
    schemaDiff = diff;

    return html;
}

function getDiffTypeLabel(type) {
    switch(type) {
        case 'CREATE': return '新增';
        case 'ALTER': return '修改';
        case 'DROP': return '删除';
        default: return type;
    }
}

function getDiffTypeBadgeClass(type) {
    switch(type) {
        case 'CREATE': return 'bg-green-100 text-green-700';
        case 'ALTER': return 'bg-yellow-100 text-yellow-700';
        case 'DROP': return 'bg-red-100 text-red-700';
        default: return 'bg-gray-100 text-gray-700';
    }
}

async function syncSchema() {
    if (!schemaDiff) return;
    const sourceID = parseInt(document.getElementById('sourceConfig').value);
    const targetSelect = document.getElementById('targetConfigs');
    const targetIDs = Array.from(targetSelect.selectedOptions).map(o => parseInt(o.value));

    if (!confirm('确定要同步表结构吗？此操作可能会修改目标数据库的表结构。')) return;

    for (const targetID of targetIDs) {
        try {
            const response = await fetch('/api/schema/sync', {
                method: 'POST',
                headers: {
                    'Content-Type': 'application/json',
                    'Authorization': `Bearer ${token}`
                },
                body: JSON.stringify({ source_id: sourceID, target_id: targetID, diff: schemaDiff })
            });
            const result = await response.json();
            if (result.message) {
                alert(`同步完成: ${result.message}`);
            }
        } catch (err) {
            alert('同步失败: ' + err.message);
        }
    }
}

async function renderDataSync(content) {
    await loadDBConfigs();
    content.innerHTML = `
        <div class="mb-6">
            <h1 class="text-2xl font-bold text-gray-800">数据同步</h1>
            <p class="text-gray-500 mt-1">将源数据库的数据同步到目标数据库</p>
        </div>
        <div class="bg-white rounded-xl shadow-sm p-6 mb-6">
            <div class="grid grid-cols-1 md:grid-cols-2 gap-6">
                <div>
                    <label class="block text-sm font-medium text-gray-700 mb-2">源数据库</label>
                    <select id="dataSourceConfig" class="w-full px-4 py-2 border border-gray-300 rounded-lg focus:ring-2 focus:ring-blue-500 outline-none" onchange="loadSourceTables()">
                        <option value="">请选择源数据库</option>
                        ${dbConfigs.map(c => `<option value="${c.id}">${c.name}</option>`).join('')}
                    </select>
                </div>
                <div>
                    <label class="block text-sm font-medium text-gray-700 mb-2">目标数据库 (可多选)</label>
                    <select id="dataTargetConfigs" multiple class="w-full px-4 py-2 border border-gray-300 rounded-lg focus:ring-2 focus:ring-blue-500 outline-none h-40">
                        ${dbConfigs.map(c => `<option value="${c.id}">${c.name}</option>`).join('')}
                    </select>
                </div>
            </div>
            <div class="mt-6">
                <label class="block text-sm font-medium text-gray-700 mb-2">选择表 (不选则同步所有表)</label>
                <select id="dataTables" multiple class="w-full px-4 py-2 border border-gray-300 rounded-lg focus:ring-2 focus:ring-blue-500 outline-none h-40">
                </select>
            </div>
            <div class="mt-6">
                <label class="flex items-center">
                    <input type="checkbox" id="deleteMissing" class="w-4 h-4 text-blue-600 border-gray-300 rounded">
                    <span class="ml-2 text-sm text-gray-700">删除目标数据库中不存在于源数据库的数据</span>
                </label>
            </div>
            <div class="mt-6">
                <button onclick="syncData()" class="bg-green-600 text-white px-6 py-2 rounded-lg font-medium hover:bg-green-700 transition flex items-center">
                    <i class="fa fa-refresh mr-2"></i>开始同步
                </button>
            </div>
        </div>
        <div id="dataSyncResult"></div>
    `;
}

async function loadSourceTables() {
    const sourceID = parseInt(document.getElementById('dataSourceConfig').value);
    const tableSelect = document.getElementById('dataTables');
    
    if (!sourceID) {
        tableSelect.innerHTML = '';
        return;
    }

    try {
        const response = await fetch(`/api/schema/tables/${sourceID}`, {
            headers: { 'Authorization': `Bearer ${token}` }
        });
        const tables = await response.json();
        tableSelect.innerHTML = tables.map(t => `<option value="${t}">${t}</option>`).join('');
    } catch (err) {
        tableSelect.innerHTML = '<option>加载失败</option>';
    }
}

async function syncData() {
    const sourceID = parseInt(document.getElementById('dataSourceConfig').value);
    const targetSelect = document.getElementById('dataTargetConfigs');
    const targetIDs = Array.from(targetSelect.selectedOptions).map(o => parseInt(o.value));
    const tableSelect = document.getElementById('dataTables');
    const tables = Array.from(tableSelect.selectedOptions).map(o => o.value);
    const deleteMissing = document.getElementById('deleteMissing').checked;

    if (!sourceID || targetIDs.length === 0) {
        alert('请选择源数据库和至少一个目标数据库');
        return;
    }

    if (!confirm('确定要同步数据吗？此操作可能会修改目标数据库的数据。')) return;

    const resultDiv = document.getElementById('dataSyncResult');
    resultDiv.innerHTML = `<div class="text-center py-8"><i class="fa fa-spinner fa-spin text-xl text-blue-600"></i> 正在同步...</div>`;

    try {
        const response = await fetch('/api/datasync/sync', {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json',
                'Authorization': `Bearer ${token}`
            },
            body: JSON.stringify({
                source_id: sourceID,
                target_ids: targetIDs,
                tables: tables,
                delete_missing: deleteMissing
            })
        });
        const results = await response.json();
        renderDataSyncResults(results, resultDiv);
    } catch (err) {
        resultDiv.innerHTML = `<div class="text-center py-8 text-red-500">同步失败: ${err.message}</div>`;
    }
}

function renderDataSyncResults(results, container) {
    if (!results || results.length === 0) {
        container.innerHTML = `<div class="text-center py-8 text-gray-500">没有同步结果</div>`;
        return;
    }

    let html = '';
    results.forEach(result => {
        html += `
            <div class="bg-white rounded-xl shadow-sm p-6 mb-6">
                <h3 class="text-lg font-semibold text-gray-800 mb-4">目标: ${result.target_name}</h3>
                ${result.error ? `<div class="text-red-500">${result.error}</div>` : renderTableResults(result.results)}
            </div>
        `;
    });
    container.innerHTML = html;
}

function renderTableResults(tableResults) {
    if (!tableResults || tableResults.length === 0) {
        return `<div class="text-gray-500">没有表数据</div>`;
    }

    let html = '<div class="overflow-x-auto">';
    html += '<table class="w-full">';
    html += `
        <thead>
            <tr class="border-b border-gray-200">
                <th class="text-left py-3 px-4 font-semibold text-gray-600">表名</th>
                <th class="text-center py-3 px-4 font-semibold text-gray-600">总数</th>
                <th class="text-center py-3 px-4 font-semibold text-gray-600">新增</th>
                <th class="text-center py-3 px-4 font-semibold text-gray-600">更新</th>
                <th class="text-center py-3 px-4 font-semibold text-gray-600">删除</th>
                <th class="text-center py-3 px-4 font-semibold text-gray-600">状态</th>
            </tr>
        </thead>
        <tbody>
    `;

    tableResults.forEach(result => {
        let statusClass = 'text-green-600';
        let statusIcon = 'fa-check-circle';
        if (result.Error) {
            statusClass = 'text-red-600';
            statusIcon = 'fa-times-circle';
        }

        html += `
            <tr class="border-b border-gray-100 hover:bg-gray-50">
                <td class="py-3 px-4 text-gray-800">${result.table}</td>
                <td class="py-3 px-4 text-center text-gray-600">${result.total}</td>
                <td class="py-3 px-4 text-center text-green-600">${result.inserted}</td>
                <td class="py-3 px-4 text-center text-blue-600">${result.updated}</td>
                <td class="py-3 px-4 text-center text-red-600">${result.deleted}</td>
                <td class="py-3 px-4 text-center ${statusClass}">
                    ${result.Error ? result.Error : `<i class="fa ${statusIcon}"></i>`}
                </td>
            </tr>
        `;
    });

    html += '</tbody></table></div>';
    return html;
}

async function renderAudit(content) {
    content.innerHTML = `
        <div class="mb-6">
            <h1 class="text-2xl font-bold text-gray-800">审计日志</h1>
            <p class="text-gray-500 mt-1">查看所有操作记录</p>
        </div>
        <div id="auditLogs"></div>
    `;
    await loadAuditLogs();
}

async function loadAuditLogs() {
    const container = document.getElementById('auditLogs');
    try {
        const response = await fetch('/api/audit/logs?limit=50', {
            headers: { 'Authorization': `Bearer ${token}` }
        });
        const logs = await response.json();
        renderAuditLogs(logs, container);
    } catch (err) {
        container.innerHTML = `<div class="text-center py-8 text-red-500">加载失败: ${err.message}</div>`;
    }
}

function renderAuditLogs(logs, container) {
    if (!logs || logs.length === 0) {
        container.innerHTML = `<div class="bg-white rounded-xl shadow-sm p-6"><div class="text-center py-8 text-gray-500">暂无审计日志</div></div>`;
        return;
    }

    let html = `
        <div class="bg-white rounded-xl shadow-sm p-6">
            <div class="overflow-x-auto">
                <table class="w-full">
                    <thead>
                        <tr class="border-b border-gray-200">
                            <th class="text-left py-3 px-4 font-semibold text-gray-600">时间</th>
                            <th class="text-left py-3 px-4 font-semibold text-gray-600">用户ID</th>
                            <th class="text-left py-3 px-4 font-semibold text-gray-600">操作</th>
                            <th class="text-left py-3 px-4 font-semibold text-gray-600">源数据库</th>
                            <th class="text-left py-3 px-4 font-semibold text-gray-600">目标数据库</th>
                            <th class="text-left py-3 px-4 font-semibold text-gray-600">详情</th>
                            <th class="text-center py-3 px-4 font-semibold text-gray-600">状态</th>
                        </tr>
                    </thead>
                    <tbody>
    `;

    logs.forEach(log => {
        let statusClass = 'text-green-600';
        if (log.status === 'FAILED') statusClass = 'text-red-600';
        if (log.status === 'PARTIAL') statusClass = 'text-yellow-600';

        html += `
            <tr class="border-b border-gray-100 hover:bg-gray-50">
                <td class="py-3 px-4 text-gray-600">${formatDate(log.created_at)}</td>
                <td class="py-3 px-4 text-gray-800">${log.user_id}</td>
                <td class="py-3 px-4">
                    <span class="px-2 py-1 text-xs rounded-full bg-blue-100 text-blue-700">${log.action}</span>
                </td>
                <td class="py-3 px-4 text-gray-600">${log.source_db || '-'}</td>
                <td class="py-3 px-4 text-gray-600">${log.target_db || '-'}</td>
                <td class="py-3 px-4 text-gray-600 max-w-xs truncate" title="${log.details}">${log.details || '-'}</td>
                <td class="py-3 px-4 text-center ${statusClass}">${log.status}</td>
            </tr>
        `;
    });

    html += '</tbody></table></div></div>';
    container.innerHTML = html;
}

function formatDate(dateStr) {
    if (!dateStr) return '';
    const date = new Date(dateStr);
    return date.toLocaleString('zh-CN');
}

init();
