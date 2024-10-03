
// Import the CSS
import './assets/css/pico.purple.min.css';
import './assets/css/siimple-icons.css';
import './assets/css/main.css';
import 'izitoast/dist/css/iziToast.min.css';

import logo from './assets/images/logo-universal.png';
import {LoadHosts, RemoveHost, SaveHost} from "../wailsjs/go/app/App";
import iziToast from "izitoast";


window.onload = function () {
    loadHosts();
};


// Setup the greet function
window.loadHosts = function () {


    try {
        LoadHosts()
            .then((result) => {

                let html = ""

                for (let i = 0; i < result.length; i++) {
                    let element = result[i];

                    html += `
                        <article class="host-article" data-id="${element.id}" data-info='${JSON.stringify(element)}'>
                            
                            <div class="host-name">
                                <span >${element.name}</span>
                            </div>
                            <div class="text-end host-buttons">
                                <button class="btn-action" onclick="editHost('${element.id}')"><i class="si si-edit"></i></button>
                                <button class="btn-action btn-danger" onclick="removeHost('${element.id}')"><i class="si si-trash"></i></button>
                            </div>
                            
                            
                        </article>
                    `
                }

                document.getElementById('host-container').innerHTML = html;
            })
            .catch((err) => {
                console.error(err);
            });
    } catch (err) {
        console.error(err);
    }
};

window.searchTerm = function (searchTerm){
    let hosts = document.querySelectorAll('.host-article');


    for (let i = 0; i < hosts.length; i++) {
        let host = hosts[i];
        let hostInfo = JSON.parse(host.dataset.info);

        if (hostInfo.name.includes(searchTerm)) {
            host.style.display = 'flex';

        } else {
            host.style.display = 'none';
        }

    }
}

window.showAside = function (){
    if(document.querySelector('aside').classList.contains('show')){
        hideAside();
        setTimeout(function (){
            document.querySelector('aside').classList.add('show');
        }, 200)
    }else{
        document.querySelector('aside').classList.add('show');
    }


    document.body.style.paddingRight = "340px";
}

window.hideAside = function (){
    document.querySelector('aside').classList.remove('show');
    document.body.style.paddingRight = "30px";
}

window.addHost = function (){
    document.querySelector('#host-form').reset();
    document.querySelector('#host-form').elements['id'].value = null;
    document.querySelector('#host-form').elements['id'].disabled = true;

    showAside();
}

window.editHost = function (hostId){

    let hostInfo = document.querySelector(`.host-article[data-id="${hostId}"]`).dataset.info;
    let host = JSON.parse(hostInfo);


    document.querySelector('#host-form').reset();
    document.querySelector('#host-form').elements['id'].value = host.id;
    document.querySelector('#host-form').elements['id'].disabled = false;

    document.querySelector('#host-form').elements['name'].value = host.name;
    document.querySelector('#host-form').elements['host'].value = host.host;
    document.querySelector('#host-form').elements['port'].value = host.port;
    document.querySelector('#host-form').elements['user'].value = host.user;
    document.querySelector('#host-form').elements['pass'].value = host.pass;



    showAside();

}

window.removeHost = function (hostId){

    iziToast.show({
        icon: 'si-question-circle',
        title: 'Are you sure?',
        message: 'The host will be removed.',
        theme: 'dark',
        position: 'center',
        close: false,
        closeOnEscape: false,
        closeOnClick: false,
        timeout: false,
        buttons: [
            ['<button>Ok</button>', function (instance, toast) {

                RemoveHost(hostId)
                    .then((result) => {
                        loadHosts(); // Carga la lista de hosts
                    })
                    .catch((err) => {
                        console.error('Error:', err);
                        // Aquí puedes mostrar un mensaje de error al usuario si lo deseas
                    });



                instance.hide({
                    transitionOut: 'fadeOutUp',
                    onClosing: function(instance, toast, closedBy){
                        console.info('closedBy: ' + closedBy); // The return will be: 'closedBy: buttonName'
                    }
                }, toast, 'buttonName');

                iziToast.show({
                    title: 'OK',
                    message: 'The host has been removed',
                    theme: 'dark',
                    position: 'center',
                })

            }, true], // true to focus
            ['<button>Close</button>', function (instance, toast) {
                instance.hide({
                    transitionOut: 'fadeOutUp',
                    onClosing: function(instance, toast, closedBy){
                        console.info('closedBy: ' + closedBy); // The return will be: 'closedBy: buttonName'
                    }
                }, toast, 'buttonName');
            }]
        ],
    });


}

document.querySelector('#host-form').addEventListener('submit', function (event){
    event.preventDefault(); // Previene el envío del formulario por defecto
    let formData = new FormData(document.querySelector('#host-form'));

    const id = formData.get('id') ? formData.get('id').toString() : "0"; // Asegúrate de que ID sea un string
    const host = formData.get('host').toString(); // Asegúrate de que Host sea un string
    const port = parseInt(formData.get('port'), 10); // Convierte el puerto a un entero

    // Validación simple para el puerto
    if (isNaN(port) || port <= 0) {
        console.error('The port must be positive.');
        return; // Sale de la función si el puerto es inválido
    }

    const name = formData.get('name').toString(); // Asegúrate de que Name sea un string
    const user = formData.get('user').toString(); // Asegúrate de que User sea un string
    const pass = formData.get('pass').toString(); // Asegúrate de que Pass sea un string

    // Llama a la función SaveHost
    SaveHost(id, host, port, name, user, pass)
        .then((result) => {
            loadHosts(); // Carga la lista de hosts
            hideAside(); // Oculta el formulario
        })
        .catch((err) => {
            console.error('Error al guardar el host:', err);
            // Aquí puedes mostrar un mensaje de error al usuario si lo deseas
        });
});
