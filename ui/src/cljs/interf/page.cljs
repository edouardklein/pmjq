(ns interf.page
  (:require
   [cljs.core.async :as async :refer [put!]]
   [reagent.core :as reagent]
   )
)


(defn send-upload!
  "Receive an uploaded file and send it"
  [upload e uploaded-file dirname]
  (swap! upload assoc-in [dirname "state"] "proc") ; Upload by input
  (put! uploaded-file [e dirname]))

(defn log-text-zone
  "A text area that automatically scrolls down when changed."
  [logs]
  (reagent/create-class
   {:display-name "log-textarea"
    :component-did-mount
    (fn []
      (let [textarea (.getElementById js/document "Logs")]
        (.log js/console "Scrolling the logs")
        (aset textarea "scrollTop" (aget textarea "scrollHeight"))))
    :reagent-render
    (fn [logs]
      (.log js/console "Rendering the logs")
      [:div  "Logs :"
       [:div
        [:textarea#Logs {:rows 10 :cols 150 :readOnly true
                         :value (:text @logs)}]]])}))


(defn page
  "Web interface content"
  [upload queues logs errors result upload-chan]
  [:div
   [:h1 "Upload page"]
   [:div
    ;; Upload buttons (name, waiting queue, button)
    (doall (for [dirname (keys @upload)]
             (let [state (get-in @upload [dirname :state])
                   num (get @queues dirname)]
               ^{:key dirname} [:div (str
                                   "Destination: "
                                   dirname
                                   (if num (str " (Queue length: " num ")"))
                                   "\t-> ")
                             [:input  {:name dirname
                                       :id dirname
                                       :type "file"
                                       :disabled (some? state)
                                       :on-change
                                       #(send-upload! upload % upload-chan dirname)}]])))]
   [:button {:type "button" :on-click #(swap! logs update-in [:show] not)}
    "Toggle logs"]
   (if (:show @logs)
     [log-text-zone logs])
   (doall
    ;; Output hyperlinks
    (for [file @result]
      ^{:key file} [:div [:a {:href file :target "_blank"}
                          (str  "Download "
                                (peek (clojure.string/split file #"/")))]]))
   (doall (for [dirname (keys @upload)]
            ;; Upload statuses
            (let [state (get-in @upload [dirname :state])
                  status (fn [s] ^{:key (str dirname "status")}
                           [:p (str dirname ": " s)])]
              (cond (= state nil) (status "Waiting for upload...")
                    (= state "proc") (status "Uploading...")
                    :else (status "Done.")))))
   (if @errors
     (doall (for [err (vals @errors)]
              ;; Show error content
              ^{:key (:command err)} [:div
                                      [:p [:font {:color "red"}
                                           (str "Error during " (:command err) ": ")]
                                       [:button
                                        {:type "button"
                                         :on-click
                                         #(swap! errors update-in
                                                 [(:command err) :show] not)}
                                        "Show"]]
                                      (if (:show err)
                                        [:textarea {:readOnly true :rows 15
                                                    :cols 100
                                                    :value (or
                                                            (:message err)
                                                            "")}])])))
     [:p "(F5 -> Refresh/Reset)"]])
